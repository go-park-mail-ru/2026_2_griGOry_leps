package usecase

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	require.NoError(t, err)
	return u.Query().Get("token")
}

type resetTestEnv struct {
	uc       *PasswordResetUsecase
	users    *MockUserRepository
	sessions *MockSessionRepository
	resets   *MockPasswordResetRepository
	verifier *MockEmailVerifier
	mailer   *MockMailer
	tx       *MockTransactor
}

func newResetTestEnv(t *testing.T) *resetTestEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	e := &resetTestEnv{users: NewMockUserRepository(ctrl), sessions: NewMockSessionRepository(ctrl), resets: NewMockPasswordResetRepository(ctrl), verifier: NewMockEmailVerifier(ctrl), mailer: NewMockMailer(ctrl), tx: NewMockTransactor(ctrl)}
	e.uc = NewPasswordResetUsecase(e.users, e.sessions, e.resets, e.verifier, e.mailer, e.tx, "http://front.test/reset-password")
	return e
}

func validReset() domain.PasswordReset {
	return domain.PasswordReset{UserID: 7, ExpiresAt: time.Now().Add(time.Hour)}
}

func expectPasswordReset(e *resetTestEnv, token, password string) {
	record := validReset()
	get := e.resets.EXPECT().Get(gomock.Any(), hashToken(token)).Return(record, nil)
	tx := expectTransaction(e.tx).After(get)
	lock := e.users.EXPECT().LockByID(gomock.Any(), int32(7)).Return(nil).After(tx)
	gomock.InOrder(lock,
		e.resets.EXPECT().GetForUpdate(gomock.Any(), hashToken(token)).Return(record, nil),
		e.users.EXPECT().UpdatePassword(gomock.Any(), int32(7), gomock.Cond(func(hash string) bool { return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil })).Return(nil),
		e.users.EXPECT().SetEmailVerified(gomock.Any(), int32(7)).Return(nil),
		e.users.EXPECT().SetPendingEmail(gomock.Any(), int32(7), nil).Return(nil),
		e.verifier.EXPECT().RevokeLinks(gomock.Any(), int32(7)).Return(nil),
		e.resets.EXPECT().UseAll(gomock.Any(), int32(7)).Return(nil),
		e.sessions.EXPECT().DeleteAll(gomock.Any(), int32(7)).Return(nil))
}

func TestRequestReset_SendsLinkWithToken(t *testing.T) {
	e := newResetTestEnv(t)
	user := domain.User{ID: 7, Email: "anna@goget.pw"}
	e.users.EXPECT().GetByEmail(gomock.Any(), user.Email).Return(user, nil)
	expectTransaction(e.tx)
	gomock.InOrder(
		e.users.EXPECT().LockByID(gomock.Any(), user.ID).Return(nil),
		e.users.EXPECT().GetByID(gomock.Any(), user.ID).Return(user, nil),
		e.resets.EXPECT().HasCreatedSince(gomock.Any(), user.ID, gomock.Any()).Return(false, nil),
	)
	var stored string
	e.resets.EXPECT().Create(gomock.Any(), gomock.Any(), user.ID, gomock.Cond(func(exp time.Time) bool { return exp.After(time.Now()) })).DoAndReturn(func(_ context.Context, hash string, _ int32, _ time.Time) error { stored = hash; return nil })
	sent := make(chan string, 1)
	e.mailer.EXPECT().SendPasswordReset(gomock.Any(), user.Email, gomock.Any()).DoAndReturn(func(_ context.Context, _ string, link string) error { sent <- link; return nil })
	require.NoError(t, e.uc.RequestReset(context.Background(), "  Anna@GoGet.pw "))
	select {
	case link := <-sent:
		require.True(t, strings.HasPrefix(link, "http://front.test/reset-password?token="))
		token := tokenFromLink(t, link)
		require.Len(t, token, 64)
		require.Equal(t, hashToken(token), stored)
		require.NotEqual(t, token, stored)
	case <-time.After(time.Second):
		t.Fatal("reset mail was not sent")
	}
}

func TestRequestReset_ByPhoneSendsToEmail(t *testing.T) {
	e := newResetTestEnv(t)
	user := domain.User{ID: 7, Email: "anna@goget.pw"}
	e.users.EXPECT().GetByPhone(gomock.Any(), "+79990000001").Return(user, nil)
	expectTransaction(e.tx)
	e.users.EXPECT().LockByID(gomock.Any(), user.ID).Return(nil)
	e.users.EXPECT().GetByID(gomock.Any(), user.ID).Return(user, nil)
	e.resets.EXPECT().HasCreatedSince(gomock.Any(), user.ID, gomock.Any()).Return(false, nil)
	e.resets.EXPECT().Create(gomock.Any(), gomock.Any(), user.ID, gomock.Any()).Return(nil)
	sent := make(chan struct{}, 1)
	e.mailer.EXPECT().SendPasswordReset(gomock.Any(), user.Email, gomock.Any()).DoAndReturn(func(context.Context, string, string) error { sent <- struct{}{}; return nil })
	require.NoError(t, e.uc.RequestReset(context.Background(), "8 (999) 000-00-01"))
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("reset mail was not sent")
	}
}

func TestRequestReset_UnknownLoginLooksTheSame(t *testing.T) {
	for _, login := range []string{"nobody@goget.pw", "+79991112233"} {
		t.Run(login, func(t *testing.T) {
			e := newResetTestEnv(t)
			if strings.Contains(login, "@") {
				e.users.EXPECT().GetByEmail(gomock.Any(), login).Return(domain.User{}, domain.ErrUserNotFound)
			} else {
				e.users.EXPECT().GetByPhone(gomock.Any(), login).Return(domain.User{}, domain.ErrUserNotFound)
			}
			require.NoError(t, e.uc.RequestReset(context.Background(), login))
		})
	}
}

func TestRequestReset_InvalidLogin(t *testing.T) {
	e := newResetTestEnv(t)
	for _, login := range []string{"timur@", "12345", ""} {
		require.ErrorIs(t, e.uc.RequestReset(context.Background(), login), domain.ErrInvalidContact)
	}
}

func TestCheckToken(t *testing.T) {
	e := newResetTestEnv(t)
	e.resets.EXPECT().Get(gomock.Any(), hashToken("token")).Return(validReset(), nil)
	e.users.EXPECT().GetByID(gomock.Any(), int32(7)).Return(domain.User{Email: "anna@goget.pw"}, nil)
	email, err := e.uc.CheckToken(context.Background(), "token")
	require.NoError(t, err)
	require.Equal(t, "anna@goget.pw", email)
}

func TestCheckToken_Invalid(t *testing.T) {
	e := newResetTestEnv(t)
	used := validReset()
	now := time.Now()
	used.UsedAt = &now
	e.resets.EXPECT().Get(gomock.Any(), hashToken("nope")).Return(domain.PasswordReset{}, domain.ErrInvalidResetToken)
	e.resets.EXPECT().Get(gomock.Any(), hashToken("used")).Return(used, nil)
	for _, token := range []string{"", "nope", "used"} {
		_, err := e.uc.CheckToken(context.Background(), token)
		require.ErrorIs(t, err, domain.ErrInvalidResetToken)
	}
}

func TestRequestReset_OnceAMinute(t *testing.T) {
	e := newResetTestEnv(t)
	user := domain.User{ID: 7, Email: "anna@goget.pw"}
	e.users.EXPECT().GetByEmail(gomock.Any(), user.Email).Return(user, nil)
	expectTransaction(e.tx)
	e.users.EXPECT().LockByID(gomock.Any(), user.ID).Return(nil)
	e.users.EXPECT().GetByID(gomock.Any(), user.ID).Return(user, nil)
	e.resets.EXPECT().HasCreatedSince(gomock.Any(), user.ID, gomock.Any()).Return(true, nil)
	require.NoError(t, e.uc.RequestReset(context.Background(), user.Email))
}

func TestResetPassword_ChangesPasswordAndEndsSessions(t *testing.T) {
	e := newResetTestEnv(t)
	expectPasswordReset(e, "token", "NewSecret1")
	require.NoError(t, e.uc.ResetPassword(context.Background(), "token", "NewSecret1"))
}

func TestResetPassword_TokenWorksOnce(t *testing.T) {
	e := newResetTestEnv(t)
	expectPasswordReset(e, "token", "NewSecret1")
	require.NoError(t, e.uc.ResetPassword(context.Background(), "token", "NewSecret1"))
	used := validReset()
	now := time.Now()
	used.UsedAt = &now
	e.resets.EXPECT().Get(gomock.Any(), hashToken("token")).Return(used, nil)
	require.ErrorIs(t, e.uc.ResetPassword(context.Background(), "token", "OtherSecret2"), domain.ErrInvalidResetToken)
}

func TestResetPassword_OldTokensStopWorking(t *testing.T) {
	e := newResetTestEnv(t)
	expectPasswordReset(e, "fresh", "NewSecret1")
	require.NoError(t, e.uc.ResetPassword(context.Background(), "fresh", "NewSecret1"))
	used := validReset()
	now := time.Now()
	used.UsedAt = &now
	e.resets.EXPECT().Get(gomock.Any(), hashToken("old")).Return(used, nil)
	require.ErrorIs(t, e.uc.ResetPassword(context.Background(), "old", "OtherSecret2"), domain.ErrInvalidResetToken)
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	e := newResetTestEnv(t)
	expired := validReset()
	expired.ExpiresAt = time.Now().Add(-time.Second)
	e.resets.EXPECT().Get(gomock.Any(), hashToken("token")).Return(expired, nil)
	require.ErrorIs(t, e.uc.ResetPassword(context.Background(), "token", "NewSecret1"), domain.ErrInvalidResetToken)
}

func TestResetPassword_UnknownOrEmptyToken(t *testing.T) {
	e := newResetTestEnv(t)
	e.resets.EXPECT().Get(gomock.Any(), hashToken("nope")).Return(domain.PasswordReset{}, domain.ErrInvalidResetToken)
	for _, token := range []string{"", "nope"} {
		require.ErrorIs(t, e.uc.ResetPassword(context.Background(), token, "NewSecret1"), domain.ErrInvalidResetToken)
	}
}

func TestResetPassword_WeakPasswordKeepsToken(t *testing.T) {
	e := newResetTestEnv(t)
	require.ErrorIs(t, e.uc.ResetPassword(context.Background(), "token", "short"), domain.ErrWeakPassword)
	expectPasswordReset(e, "token", "NewSecret1")
	require.NoError(t, e.uc.ResetPassword(context.Background(), "token", "NewSecret1"))
}

func TestResetPassword_LocksUserBeforeToken(t *testing.T) {
	e := newResetTestEnv(t)
	expectPasswordReset(e, "token", "NewSecret1")
	require.NoError(t, e.uc.ResetPassword(context.Background(), "token", "NewSecret1"))
}

func TestResetPassword_ExpiredTokenSkipsTransaction(t *testing.T) {
	e := newResetTestEnv(t)
	expired := validReset()
	expired.ExpiresAt = time.Now().Add(-time.Second)
	e.resets.EXPECT().Get(gomock.Any(), hashToken("token")).Return(expired, nil)
	require.ErrorIs(t, e.uc.ResetPassword(context.Background(), "token", "NewSecret1"), domain.ErrInvalidResetToken)
}

func TestRequestReset_EmailChangedBeforeLock(t *testing.T) {
	e := newResetTestEnv(t)
	e.users.EXPECT().GetByEmail(gomock.Any(), "anna@goget.pw").Return(domain.User{ID: 7, Email: "anna@goget.pw"}, nil)
	expectTransaction(e.tx)
	e.users.EXPECT().LockByID(gomock.Any(), int32(7)).Return(nil)
	e.users.EXPECT().GetByID(gomock.Any(), int32(7)).Return(domain.User{ID: 7, Email: "changed@goget.pw"}, nil)
	require.NoError(t, e.uc.RequestReset(context.Background(), "anna@goget.pw"))
}
