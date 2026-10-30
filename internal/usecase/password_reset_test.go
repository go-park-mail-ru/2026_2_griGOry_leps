package usecase

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type resetRecord struct {
	userID    int32
	createdAt time.Time
	expiresAt time.Time
	usedAt    *time.Time
}

type resetRepoFake struct {
	mu     sync.Mutex
	resets map[string]*resetRecord
}

func newResetRepoFake() *resetRepoFake {
	return &resetRepoFake{resets: make(map[string]*resetRecord)}
}

func (r *resetRepoFake) Create(_ context.Context, tokenHash string, userID int32, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resets[tokenHash] = &resetRecord{userID: userID, createdAt: time.Now(), expiresAt: expiresAt}
	return nil
}

func (r *resetRepoFake) HasCreatedSince(_ context.Context, userID int32, since time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.resets {
		if rec.userID == userID && rec.createdAt.After(since) {
			return true, nil
		}
	}
	return false, nil
}

func (r *resetRepoFake) Get(ctx context.Context, tokenHash string) (domain.PasswordReset, error) {
	return r.GetForUpdate(ctx, tokenHash)
}

func (r *resetRepoFake) GetForUpdate(_ context.Context, tokenHash string) (domain.PasswordReset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.resets[tokenHash]
	if !ok {
		return domain.PasswordReset{}, domain.ErrInvalidResetToken
	}
	return domain.PasswordReset{UserID: rec.userID, ExpiresAt: rec.expiresAt, UsedAt: rec.usedAt}, nil
}

func (r *resetRepoFake) UseAll(_ context.Context, userID int32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, rec := range r.resets {
		if rec.userID == userID && rec.usedAt == nil {
			rec.usedAt = &now
		}
	}
	return nil
}

type sentMail struct {
	to   string
	link string
}

type mailerFake struct {
	sent chan sentMail
}

func (m mailerFake) SendPasswordReset(_ context.Context, to, link string) error {
	m.sent <- sentMail{to: to, link: link}
	return nil
}

func (m mailerFake) SendEmailVerification(_ context.Context, to, link string) error {
	m.sent <- sentMail{to: to, link: link}
	return nil
}

type resetEnv struct {
	uc       *PasswordResetUsecase
	users    *userRepoFake
	sessions *sessionRepoFake
	resets   *resetRepoFake
	mails    chan sentMail
	userID   int32
}

func newResetEnv(t *testing.T) resetEnv {
	t.Helper()
	users := newUserRepoFake()
	sessions := newSessionRepoFake()
	resets := newResetRepoFake()
	mails := make(chan sentMail, 10)

	hash, err := bcrypt.GenerateFromPassword([]byte("Secret123"), bcrypt.MinCost)
	require.NoError(t, err)
	user, err := users.Create(context.Background(), "anna@goget.pw", string(hash), "Анна", "+79990000001")
	require.NoError(t, err)

	return resetEnv{
		uc:       NewPasswordResetUsecase(users, sessions, resets, mailerFake{sent: mails}, txFake{}, "http://front.test/reset-password"),
		users:    users,
		sessions: sessions,
		resets:   resets,
		mails:    mails,
		userID:   user.ID,
	}
}

func (env resetEnv) waitMail(t *testing.T) sentMail {
	t.Helper()
	select {
	case m := <-env.mails:
		return m
	case <-time.After(time.Second):
		t.Fatal("reset mail was not sent")
		return sentMail{}
	}
}

func (env resetEnv) requireNoMail(t *testing.T) {
	t.Helper()
	select {
	case m := <-env.mails:
		t.Fatalf("unexpected mail to %s", m.to)
	case <-time.After(50 * time.Millisecond):
	}
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	require.NoError(t, err)
	return u.Query().Get("token")
}

func (env resetEnv) requestToken(t *testing.T) string {
	t.Helper()
	require.NoError(t, env.uc.RequestReset(context.Background(), "anna@goget.pw"))
	return tokenFromLink(t, env.waitMail(t).link)
}

func TestRequestReset_SendsLinkWithToken(t *testing.T) {
	env := newResetEnv(t)

	require.NoError(t, env.uc.RequestReset(context.Background(), "  Anna@GoGet.pw "))

	m := env.waitMail(t)
	require.Equal(t, "anna@goget.pw", m.to)
	require.True(t, strings.HasPrefix(m.link, "http://front.test/reset-password?token="))
	token := tokenFromLink(t, m.link)
	require.Len(t, token, 64)
	require.Contains(t, env.resets.resets, hashToken(token))
	require.NotContains(t, env.resets.resets, token)
}

func TestRequestReset_ByPhoneSendsToEmail(t *testing.T) {
	env := newResetEnv(t)

	require.NoError(t, env.uc.RequestReset(context.Background(), "8 (999) 000-00-01"))

	require.Equal(t, "anna@goget.pw", env.waitMail(t).to)
}

func TestRequestReset_UnknownLoginLooksTheSame(t *testing.T) {
	for _, login := range []string{"nobody@goget.pw", "+79991112233"} {
		t.Run(login, func(t *testing.T) {
			env := newResetEnv(t)

			require.NoError(t, env.uc.RequestReset(context.Background(), login))

			env.requireNoMail(t)
			require.Empty(t, env.resets.resets)
		})
	}
}

func TestRequestReset_InvalidLogin(t *testing.T) {
	env := newResetEnv(t)

	for _, login := range []string{"timur@", "12345", ""} {
		require.ErrorIs(t, env.uc.RequestReset(context.Background(), login), domain.ErrInvalidContact, login)
	}
}

func TestCheckToken(t *testing.T) {
	env := newResetEnv(t)
	token := env.requestToken(t)

	email, err := env.uc.CheckToken(context.Background(), token)

	require.NoError(t, err)
	require.Equal(t, "anna@goget.pw", email)
}

func TestCheckToken_Invalid(t *testing.T) {
	env := newResetEnv(t)
	used := env.requestToken(t)
	require.NoError(t, env.uc.ResetPassword(context.Background(), used, "NewSecret1"))

	for _, token := range []string{"", "nope", used} {
		_, err := env.uc.CheckToken(context.Background(), token)
		require.ErrorIs(t, err, domain.ErrInvalidResetToken, token)
	}
}

func TestRequestReset_OnceAMinute(t *testing.T) {
	env := newResetEnv(t)
	env.requestToken(t)

	require.NoError(t, env.uc.RequestReset(context.Background(), "anna@goget.pw"))

	env.requireNoMail(t)
	require.Len(t, env.resets.resets, 1)
}

func TestResetPassword_ChangesPasswordAndEndsSessions(t *testing.T) {
	env := newResetEnv(t)
	ctx := context.Background()
	addSessionFor(t, env.sessions, "laptop", env.userID)
	addSessionFor(t, env.sessions, "phone", env.userID)
	addSessionFor(t, env.sessions, "stranger", env.userID+100)
	token := env.requestToken(t)

	require.NoError(t, env.uc.ResetPassword(ctx, token, "NewSecret1"))

	user, err := env.users.GetByID(ctx, env.userID)
	require.NoError(t, err)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("NewSecret1")))
	for _, id := range []string{"laptop", "phone"} {
		_, err := env.sessions.GetByID(ctx, id)
		require.ErrorIs(t, err, domain.ErrSessionNotFound, id)
	}
	_, err = env.sessions.GetByID(ctx, "stranger")
	require.NoError(t, err)
}

func TestResetPassword_TokenWorksOnce(t *testing.T) {
	env := newResetEnv(t)
	token := env.requestToken(t)
	require.NoError(t, env.uc.ResetPassword(context.Background(), token, "NewSecret1"))

	err := env.uc.ResetPassword(context.Background(), token, "OtherSecret2")

	require.ErrorIs(t, err, domain.ErrInvalidResetToken)
}

func TestResetPassword_OldTokensStopWorking(t *testing.T) {
	env := newResetEnv(t)
	old := env.requestToken(t)
	for _, rec := range env.resets.resets {
		rec.createdAt = rec.createdAt.Add(-2 * resetCooldown)
	}
	fresh := env.requestToken(t)
	require.NotEqual(t, old, fresh)

	require.NoError(t, env.uc.ResetPassword(context.Background(), fresh, "NewSecret1"))

	require.ErrorIs(t, env.uc.ResetPassword(context.Background(), old, "OtherSecret2"), domain.ErrInvalidResetToken)
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	env := newResetEnv(t)
	token := env.requestToken(t)
	env.resets.resets[hashToken(token)].expiresAt = time.Now().Add(-time.Second)

	err := env.uc.ResetPassword(context.Background(), token, "NewSecret1")

	require.ErrorIs(t, err, domain.ErrInvalidResetToken)
}

func TestResetPassword_UnknownOrEmptyToken(t *testing.T) {
	env := newResetEnv(t)

	require.ErrorIs(t, env.uc.ResetPassword(context.Background(), "", "NewSecret1"), domain.ErrInvalidResetToken)
	require.ErrorIs(t, env.uc.ResetPassword(context.Background(), "nope", "NewSecret1"), domain.ErrInvalidResetToken)
}

func TestResetPassword_WeakPasswordKeepsToken(t *testing.T) {
	env := newResetEnv(t)
	token := env.requestToken(t)

	require.ErrorIs(t, env.uc.ResetPassword(context.Background(), token, "short"), domain.ErrWeakPassword)

	require.NoError(t, env.uc.ResetPassword(context.Background(), token, "NewSecret1"))
}

func addSessionFor(t *testing.T, sessions *sessionRepoFake, id string, userID int32) {
	t.Helper()
	require.NoError(t, sessions.Create(context.Background(), id, userID, time.Now().Add(time.Hour)))
}

type lockLog struct {
	mu     sync.Mutex
	events []string
}

func (l *lockLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

type lockingUsers struct {
	*userRepoFake
	log *lockLog
}

func (u lockingUsers) LockByID(ctx context.Context, id int32) error {
	u.log.add("user")
	return u.userRepoFake.LockByID(ctx, id)
}

type lockingResets struct {
	*resetRepoFake
	log *lockLog
}

func (r lockingResets) GetForUpdate(ctx context.Context, tokenHash string) (domain.PasswordReset, error) {
	r.log.add("token")
	return r.resetRepoFake.GetForUpdate(ctx, tokenHash)
}

func TestResetPassword_LocksUserBeforeToken(t *testing.T) {
	env := newResetEnv(t)
	token := env.requestToken(t)
	log := &lockLog{}
	uc := NewPasswordResetUsecase(lockingUsers{env.users, log}, env.sessions, lockingResets{env.resets, log},
		mailerFake{sent: env.mails}, txFake{}, "http://front.test/reset-password")

	require.NoError(t, uc.ResetPassword(context.Background(), token, "NewSecret1"))

	require.Equal(t, []string{"user", "token"}, log.events)
}

func TestResetPassword_ExpiredTokenSkipsTransaction(t *testing.T) {
	env := newResetEnv(t)
	token := env.requestToken(t)
	env.resets.resets[hashToken(token)].expiresAt = time.Now().Add(-time.Second)
	log := &lockLog{}
	uc := NewPasswordResetUsecase(lockingUsers{env.users, log}, env.sessions, lockingResets{env.resets, log},
		mailerFake{sent: env.mails}, txFake{}, "http://front.test/reset-password")

	require.ErrorIs(t, uc.ResetPassword(context.Background(), token, "NewSecret1"), domain.ErrInvalidResetToken)
	require.Empty(t, log.events)
}

type emailChangedRepo struct {
	*userRepoFake
}

func (r emailChangedRepo) GetByID(ctx context.Context, id int32) (domain.User, error) {
	user, err := r.userRepoFake.GetByID(ctx, id)
	user.Email = "changed@goget.pw"
	return user, err
}

func TestRequestReset_EmailChangedBeforeLock(t *testing.T) {
	env := newResetEnv(t)
	uc := NewPasswordResetUsecase(emailChangedRepo{env.users}, env.sessions, env.resets,
		mailerFake{sent: env.mails}, txFake{}, "http://front.test/reset-password")

	require.NoError(t, uc.RequestReset(context.Background(), "anna@goget.pw"))

	env.requireNoMail(t)
	require.Empty(t, env.resets.resets)
}
