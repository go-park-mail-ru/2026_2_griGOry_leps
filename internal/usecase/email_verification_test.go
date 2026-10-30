package usecase

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type verifierFake struct {
	mu   sync.Mutex
	sent []sentMail
}

func (v *verifierFake) SendLink(_ context.Context, _ int32, email string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.sent = append(v.sent, sentMail{to: email})
	return nil
}

func (v *verifierFake) emails() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	emails := make([]string, 0, len(v.sent))
	for _, m := range v.sent {
		emails = append(emails, m.to)
	}
	return emails
}

type verificationRecord struct {
	domain.EmailVerification
	createdAt time.Time
}

type verificationRepoFake struct {
	mu      sync.Mutex
	records map[string]*verificationRecord
}

func newVerificationRepoFake() *verificationRepoFake {
	return &verificationRepoFake{records: make(map[string]*verificationRecord)}
}

func (r *verificationRepoFake) Create(_ context.Context, tokenHash string, userID int32, email string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records[tokenHash] = &verificationRecord{
		EmailVerification: domain.EmailVerification{UserID: userID, Email: email, ExpiresAt: expiresAt},
		createdAt:         time.Now(),
	}
	return nil
}

func (r *verificationRepoFake) LastCreatedAt(_ context.Context, userID int32) (time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var last time.Time
	for _, rec := range r.records {
		if rec.UserID == userID && rec.createdAt.After(last) {
			last = rec.createdAt
		}
	}
	return last, nil
}

func (r *verificationRepoFake) Get(ctx context.Context, tokenHash string) (domain.EmailVerification, error) {
	return r.GetForUpdate(ctx, tokenHash)
}

func (r *verificationRepoFake) GetForUpdate(_ context.Context, tokenHash string) (domain.EmailVerification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.records[tokenHash]
	if !ok {
		return domain.EmailVerification{}, domain.ErrInvalidVerifyToken
	}
	return rec.EmailVerification, nil
}

func (r *verificationRepoFake) UseAll(_ context.Context, userID int32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, rec := range r.records {
		if rec.UserID == userID && rec.UsedAt == nil {
			rec.UsedAt = &now
		}
	}
	return nil
}

func (r *verificationRepoFake) backdate(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.records {
		rec.createdAt = rec.createdAt.Add(-d)
		rec.ExpiresAt = rec.ExpiresAt.Add(-d)
	}
}

type verifyEnv struct {
	uc            *EmailVerificationUsecase
	users         *userRepoFake
	verifications *verificationRepoFake
	resets        *resetRepoFake
	mails         chan sentMail
	userID        int32
}

func newVerifyEnv(t *testing.T) verifyEnv {
	t.Helper()
	users := newUserRepoFake()
	verifications := newVerificationRepoFake()
	resets := newResetRepoFake()
	mails := make(chan sentMail, 10)

	user, err := users.Create(context.Background(), "anna@goget.pw", "hash", "Анна", "+79990000001")
	require.NoError(t, err)

	return verifyEnv{
		uc:            NewEmailVerificationUsecase(users, verifications, resets, mailerFake{sent: mails}, txFake{}, "http://front.test/verify-email"),
		users:         users,
		verifications: verifications,
		resets:        resets,
		mails:         mails,
		userID:        user.ID,
	}
}

func (env verifyEnv) waitToken(t *testing.T, to string) string {
	t.Helper()
	select {
	case m := <-env.mails:
		require.Equal(t, to, m.to)
		require.True(t, strings.HasPrefix(m.link, "http://front.test/verify-email?token="), m.link)
		u, err := url.Parse(m.link)
		require.NoError(t, err)
		return u.Query().Get("token")
	case <-time.After(time.Second):
		t.Fatal("verification mail was not sent")
		return ""
	}
}

func (env verifyEnv) user(t *testing.T) domain.User {
	t.Helper()
	user, err := env.users.GetByID(context.Background(), env.userID)
	require.NoError(t, err)
	return user
}

func (env verifyEnv) verify(t *testing.T) {
	t.Helper()
	require.NoError(t, env.users.SetEmailVerified(context.Background(), env.userID))
}

func (env verifyEnv) requestChange(t *testing.T, email string) string {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, env.users.SetPendingEmail(ctx, env.userID, &email))
	require.NoError(t, env.uc.SendLink(ctx, env.userID, email))
	return env.waitToken(t, email)
}

func TestVerify_LinkConfirmsEmailOnce(t *testing.T) {
	env := newVerifyEnv(t)
	ctx := context.Background()

	require.NoError(t, env.uc.SendLink(ctx, env.userID, "anna@goget.pw"))
	token := env.waitToken(t, "anna@goget.pw")

	res, err := env.uc.Verify(ctx, token)
	require.NoError(t, err)
	require.Equal(t, domain.EmailVerifyResult{Email: "anna@goget.pw"}, res)
	require.True(t, env.user(t).EmailVerified)

	_, err = env.uc.Verify(ctx, token)
	require.ErrorIs(t, err, domain.ErrEmailAlreadyVerified)
}

func TestVerify_UnknownOrEmptyToken(t *testing.T) {
	env := newVerifyEnv(t)

	_, err := env.uc.Verify(context.Background(), "")
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	_, err = env.uc.Verify(context.Background(), "nope")
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	require.False(t, env.user(t).EmailVerified)
}

func TestVerify_ExpiredTokenTellsWhereToResend(t *testing.T) {
	env := newVerifyEnv(t)
	ctx := context.Background()

	require.NoError(t, env.uc.SendLink(ctx, env.userID, "anna@goget.pw"))
	token := env.waitToken(t, "anna@goget.pw")
	env.verifications.backdate(verifyTokenTTL + time.Minute)

	res, err := env.uc.Verify(ctx, token)
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	require.Equal(t, "anna@goget.pw", res.Email)
	require.False(t, env.user(t).EmailVerified)
}

func TestVerify_OldLinkDiesAfterUnverifiedEmailReplaced(t *testing.T) {
	env := newVerifyEnv(t)
	ctx := context.Background()

	require.NoError(t, env.uc.SendLink(ctx, env.userID, "anna@goget.pw"))
	token := env.waitToken(t, "anna@goget.pw")

	email := "new@goget.pw"
	require.NoError(t, env.users.UpdateProfile(ctx, env.userID, domain.ProfileUpdate{Email: &email}))

	_, err := env.uc.Verify(ctx, token)
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	require.False(t, env.user(t).EmailVerified)
}

func TestVerify_PendingEmailReplacesOldOne(t *testing.T) {
	env := newVerifyEnv(t)
	ctx := context.Background()
	env.verify(t)
	require.NoError(t, env.resets.Create(ctx, hashToken("reset"), env.userID, time.Now().Add(time.Hour)))

	token := env.requestChange(t, "new@goget.pw")
	require.Equal(t, "anna@goget.pw", env.user(t).Email)

	res, err := env.uc.Verify(ctx, token)
	require.NoError(t, err)
	require.Equal(t, domain.EmailVerifyResult{Email: "new@goget.pw", Changed: true}, res)

	user := env.user(t)
	require.Equal(t, "new@goget.pw", user.Email)
	require.Empty(t, user.PendingEmail)
	require.True(t, user.EmailVerified)

	reset, err := env.resets.Get(ctx, hashToken("reset"))
	require.NoError(t, err)
	require.NotNil(t, reset.UsedAt)
}

func TestVerify_PendingEmailTakenMeanwhile(t *testing.T) {
	env := newVerifyEnv(t)
	ctx := context.Background()
	env.verify(t)
	token := env.requestChange(t, "new@goget.pw")

	_, err := env.users.Create(ctx, "new@goget.pw", "hash", "Борис", "+79990000002")
	require.NoError(t, err)

	_, err = env.uc.Verify(ctx, token)
	require.ErrorIs(t, err, domain.ErrEmailTaken)
	require.Equal(t, "anna@goget.pw", env.user(t).Email)
}

func TestCancelEmailChange(t *testing.T) {
	env := newVerifyEnv(t)
	ctx := context.Background()
	env.verify(t)
	token := env.requestChange(t, "new@goget.pw")

	require.NoError(t, env.uc.CancelEmailChange(ctx, env.userID))
	require.Empty(t, env.user(t).PendingEmail)

	res, err := env.uc.Verify(ctx, token)
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	require.Empty(t, res.Email)
	require.Equal(t, "anna@goget.pw", env.user(t).Email)

	require.ErrorIs(t, env.uc.CancelEmailChange(ctx, env.userID), domain.ErrNoPendingEmail)
}

func TestResend_CooldownThenSends(t *testing.T) {
	env := newVerifyEnv(t)
	ctx := context.Background()

	email, wait, err := env.uc.Resend(ctx, env.userID)
	require.NoError(t, err)
	require.Zero(t, wait)
	require.Equal(t, "anna@goget.pw", email)
	env.waitToken(t, "anna@goget.pw")

	_, wait, err = env.uc.Resend(ctx, env.userID)
	require.NoError(t, err)
	require.Greater(t, wait, 50*time.Second)
	require.LessOrEqual(t, wait, verifyCooldown)
	require.Empty(t, env.mails)

	env.verifications.backdate(verifyCooldown)
	_, wait, err = env.uc.Resend(ctx, env.userID)
	require.NoError(t, err)
	require.Zero(t, wait)
	env.waitToken(t, "anna@goget.pw")
}

func TestResend_GoesToPendingEmail(t *testing.T) {
	env := newVerifyEnv(t)
	env.verify(t)
	env.requestChange(t, "new@goget.pw")
	env.verifications.backdate(verifyCooldown)

	email, wait, err := env.uc.Resend(context.Background(), env.userID)
	require.NoError(t, err)
	require.Zero(t, wait)
	require.Equal(t, "new@goget.pw", email)
	env.waitToken(t, "new@goget.pw")
}

func TestResend_AlreadyVerified(t *testing.T) {
	env := newVerifyEnv(t)
	env.verify(t)

	_, _, err := env.uc.Resend(context.Background(), env.userID)
	require.ErrorIs(t, err, domain.ErrEmailAlreadyVerified)
	require.Empty(t, env.mails)
}

func TestResendByToken(t *testing.T) {
	env := newVerifyEnv(t)
	ctx := context.Background()

	require.NoError(t, env.uc.SendLink(ctx, env.userID, "anna@goget.pw"))
	token := env.waitToken(t, "anna@goget.pw")
	env.verifications.backdate(verifyTokenTTL)

	email, wait, err := env.uc.ResendByToken(ctx, token)
	require.NoError(t, err)
	require.Zero(t, wait)
	require.Equal(t, "anna@goget.pw", email)
	env.waitToken(t, "anna@goget.pw")

	_, _, err = env.uc.ResendByToken(ctx, "nope")
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)

	email2 := "new@goget.pw"
	require.NoError(t, env.users.UpdateProfile(ctx, env.userID, domain.ProfileUpdate{Email: &email2}))
	_, _, err = env.uc.ResendByToken(ctx, token)
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
}

func TestRegister_SendsVerificationLink(t *testing.T) {
	verifier := &verifierFake{}
	uc := NewAuthUsecase(newUserRepoFake(), newSessionRepoFake(), verifier, txFake{})

	user, _, err := uc.Register(context.Background(), "Anna@GoGet.pw", "Secret123", "Анна", "+79990000001")
	require.NoError(t, err)
	require.False(t, user.EmailVerified)
	require.Equal(t, []string{"anna@goget.pw"}, verifier.emails())
}

func TestUpdateProfile_UnverifiedEmailIsReplaced(t *testing.T) {
	env := newProfileEnv(t)

	p, err := env.uc.UpdateProfile(context.Background(), env.userID, domain.ProfileUpdate{Email: ptr("new@goget.pw")})
	require.NoError(t, err)
	require.Equal(t, "new@goget.pw", p.Email)
	require.Empty(t, p.PendingEmail)
	require.False(t, p.EmailVerified)
	require.Equal(t, []string{"new@goget.pw"}, env.verifier.emails())
}

func TestUpdateProfile_VerifiedEmailWaitsForConfirmation(t *testing.T) {
	env := newProfileEnv(t)
	ctx := context.Background()
	require.NoError(t, env.users.SetEmailVerified(ctx, env.userID))
	env.addResetToken(t, "link")

	p, err := env.uc.UpdateProfile(ctx, env.userID, domain.ProfileUpdate{Email: ptr("new@goget.pw")})
	require.NoError(t, err)
	require.Equal(t, "anna@goget.pw", p.Email)
	require.Equal(t, "new@goget.pw", p.PendingEmail)
	require.True(t, p.EmailVerified)
	require.False(t, env.resetTokenUsed(t, "link"))
	require.Equal(t, []string{"new@goget.pw"}, env.verifier.emails())

	_, err = env.uc.UpdateProfile(ctx, env.userID, domain.ProfileUpdate{Email: ptr("new@goget.pw")})
	require.NoError(t, err)
	require.Len(t, env.verifier.emails(), 1)

	p, err = env.uc.UpdateProfile(ctx, env.userID, domain.ProfileUpdate{Email: ptr("anna@goget.pw")})
	require.NoError(t, err)
	require.Empty(t, p.PendingEmail)
}

func TestUpdateProfile_PendingEmailMustBeFree(t *testing.T) {
	env := newProfileEnv(t)
	ctx := context.Background()
	require.NoError(t, env.users.SetEmailVerified(ctx, env.userID))
	_, err := env.users.Create(ctx, "boris@goget.pw", "hash", "Борис", "+79990000002")
	require.NoError(t, err)

	_, err = env.uc.UpdateProfile(ctx, env.userID, domain.ProfileUpdate{Email: ptr("boris@goget.pw")})
	require.ErrorIs(t, err, domain.ErrEmailTaken)
	require.Empty(t, env.verifier.emails())
}

func TestResetPassword_ConfirmsEmail(t *testing.T) {
	env := newResetEnv(t)
	ctx := context.Background()

	require.NoError(t, env.uc.RequestReset(ctx, "anna@goget.pw"))
	link := env.waitMail(t).link
	u, err := url.Parse(link)
	require.NoError(t, err)

	require.NoError(t, env.uc.ResetPassword(ctx, u.Query().Get("token"), "NewPass123"))

	user, err := env.users.GetByID(ctx, env.userID)
	require.NoError(t, err)
	require.True(t, user.EmailVerified)
}
