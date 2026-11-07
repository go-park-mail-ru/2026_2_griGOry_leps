package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type verifyTestEnv struct {
	uc     *EmailVerificationUsecase
	users  *MockUserRepository
	repo   *MockEmailVerificationRepository
	resets *MockPasswordResetRepository
	mailer *MockMailer
	tx     *MockTransactor
}

func newVerifyTestEnv(t *testing.T) *verifyTestEnv {
	t.Helper()
	c := gomock.NewController(t)
	e := &verifyTestEnv{users: NewMockUserRepository(c), repo: NewMockEmailVerificationRepository(c), resets: NewMockPasswordResetRepository(c), mailer: NewMockMailer(c), tx: NewMockTransactor(c)}
	e.uc = NewEmailVerificationUsecase(e.users, e.repo, e.resets, e.mailer, e.tx, "http://front.test/verify-email")
	return e
}

func verification(email string) domain.EmailVerification {
	return domain.EmailVerification{UserID: 7, Email: email, ExpiresAt: time.Now().Add(time.Hour)}
}

func expectVerify(e *verifyTestEnv, token string, v domain.EmailVerification, user domain.User) {
	get := e.repo.EXPECT().Get(gomock.Any(), hashToken(token)).Return(v, nil)
	tx := expectTransaction(e.tx).After(get)
	gomock.InOrder(
		tx,
		e.users.EXPECT().LockByID(gomock.Any(), int32(7)).Return(nil),
		e.repo.EXPECT().GetForUpdate(gomock.Any(), hashToken(token)).Return(v, nil),
		e.users.EXPECT().GetByID(gomock.Any(), int32(7)).Return(user, nil),
	)
}

func expectResendUser(e *verifyTestEnv, user domain.User) {
	expectTransaction(e.tx)
	gomock.InOrder(
		e.users.EXPECT().LockByID(gomock.Any(), int32(7)).Return(nil),
		e.users.EXPECT().GetByID(gomock.Any(), int32(7)).Return(user, nil),
	)
}

func expectIssueMail(e *verifyTestEnv, email string) <-chan string {
	sent := make(chan string, 1)
	var stored string
	gomock.InOrder(
		e.repo.EXPECT().LastCreatedAt(gomock.Any(), int32(7)).Return(time.Time{}, nil),
		e.repo.EXPECT().UseAll(gomock.Any(), int32(7)).Return(nil),
		e.repo.EXPECT().Create(gomock.Any(), gomock.Any(), int32(7), email, gomock.Cond(func(exp time.Time) bool { return exp.After(time.Now()) })).DoAndReturn(func(_ context.Context, hash string, _ int32, _ string, _ time.Time) error { stored = hash; return nil }),
	)
	e.mailer.EXPECT().SendEmailVerification(gomock.Any(), email, gomock.Cond(func(link string) bool {
		const prefix = "http://front.test/verify-email?token="
		return len(link) == len(prefix)+64 && link[:len(prefix)] == prefix && hashToken(link[len(prefix):]) == stored
	})).DoAndReturn(func(_ context.Context, _ string, link string) error { sent <- link; return nil })
	return sent
}

func waitVerifyMail(t *testing.T, sent <-chan string) string {
	t.Helper()
	select {
	case link := <-sent:
		return tokenFromLink(t, link)
	case <-time.After(time.Second):
		t.Fatal("verification mail was not sent")
		return ""
	}
}

func TestVerify_LinkConfirmsEmailOnce(t *testing.T) {
	e := newVerifyTestEnv(t)
	u := profileUser()
	v := verification(u.Email)
	expectVerify(e, "token", v, u)
	e.users.EXPECT().SetEmailVerified(gomock.Any(), u.ID).Return(nil)
	e.repo.EXPECT().UseAll(gomock.Any(), u.ID).Return(nil)
	res, err := e.uc.Verify(context.Background(), "token")
	require.NoError(t, err)
	require.Equal(t, domain.EmailVerifyResult{Email: u.Email}, res)
	now := time.Now()
	v.UsedAt = &now
	u.EmailVerified = true
	expectVerify(e, "token", v, u)
	_, err = e.uc.Verify(context.Background(), "token")
	require.ErrorIs(t, err, domain.ErrEmailAlreadyVerified)
}

func TestVerify_UnknownOrEmptyToken(t *testing.T) {
	e := newVerifyTestEnv(t)
	e.repo.EXPECT().Get(gomock.Any(), hashToken("nope")).Return(domain.EmailVerification{}, domain.ErrInvalidVerifyToken)
	for _, token := range []string{"", "nope"} {
		_, err := e.uc.Verify(context.Background(), token)
		require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	}
}

func TestVerify_ExpiredTokenTellsWhereToResend(t *testing.T) {
	e := newVerifyTestEnv(t)
	u := profileUser()
	v := verification(u.Email)
	v.ExpiresAt = time.Now().Add(-time.Second)
	expectVerify(e, "token", v, u)
	res, err := e.uc.Verify(context.Background(), "token")
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	require.Equal(t, u.Email, res.Email)
}

func TestVerify_OldLinkDiesAfterUnverifiedEmailReplaced(t *testing.T) {
	e := newVerifyTestEnv(t)
	u := profileUser()
	v := verification(u.Email)
	u.Email = "new@goget.pw"
	expectVerify(e, "old", v, u)
	_, err := e.uc.Verify(context.Background(), "old")
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
}

func TestVerify_PendingEmailReplacesOldOne(t *testing.T) {
	e := newVerifyTestEnv(t)
	u := profileUser()
	u.EmailVerified = true
	u.PendingEmail = "new@goget.pw"
	expectVerify(e, "token", verification(u.PendingEmail), u)
	gomock.InOrder(
		e.users.EXPECT().ConfirmPendingEmail(gomock.Any(), u.ID).Return(nil),
		e.resets.EXPECT().UseAll(gomock.Any(), u.ID).Return(nil),
		e.repo.EXPECT().UseAll(gomock.Any(), u.ID).Return(nil),
	)
	res, err := e.uc.Verify(context.Background(), "token")
	require.NoError(t, err)
	require.Equal(t, domain.EmailVerifyResult{Email: u.PendingEmail, Changed: true}, res)
}

func TestVerify_PendingEmailTakenMeanwhile(t *testing.T) {
	e := newVerifyTestEnv(t)
	u := profileUser()
	u.EmailVerified = true
	u.PendingEmail = "new@goget.pw"
	expectVerify(e, "token", verification(u.PendingEmail), u)
	e.users.EXPECT().ConfirmPendingEmail(gomock.Any(), u.ID).Return(domain.ErrEmailTaken)
	_, err := e.uc.Verify(context.Background(), "token")
	require.ErrorIs(t, err, domain.ErrEmailTaken)
}

func TestCancelEmailChange(t *testing.T) {
	e := newVerifyTestEnv(t)
	u := profileUser()
	u.EmailVerified = true
	u.PendingEmail = "new@goget.pw"
	expectResendUser(e, u)
	gomock.InOrder(
		e.users.EXPECT().SetPendingEmail(gomock.Any(), u.ID, nil).Return(nil),
		e.repo.EXPECT().UseAll(gomock.Any(), u.ID).Return(nil),
	)
	require.NoError(t, e.uc.CancelEmailChange(context.Background(), u.ID))
	u.PendingEmail = ""
	expectResendUser(e, u)
	require.ErrorIs(t, e.uc.CancelEmailChange(context.Background(), u.ID), domain.ErrNoPendingEmail)
}

func TestResend_CooldownThenSends(t *testing.T) {
	e := newVerifyTestEnv(t)
	u := profileUser()
	expectResendUser(e, u)
	sent := expectIssueMail(e, u.Email)
	email, wait, err := e.uc.Resend(context.Background(), u.ID)
	require.NoError(t, err)
	require.Zero(t, wait)
	require.Equal(t, u.Email, email)
	waitVerifyMail(t, sent)
	expectResendUser(e, u)
	e.repo.EXPECT().LastCreatedAt(gomock.Any(), u.ID).Return(time.Now(), nil)
	_, wait, err = e.uc.Resend(context.Background(), u.ID)
	require.NoError(t, err)
	require.Greater(t, wait, 50*time.Second)
	require.LessOrEqual(t, wait, verifyCooldown)
	expectResendUser(e, u)
	sent = expectIssueMail(e, u.Email)
	_, wait, err = e.uc.Resend(context.Background(), u.ID)
	require.NoError(t, err)
	require.Zero(t, wait)
	waitVerifyMail(t, sent)
}

func TestResend_GoesToPendingEmail(t *testing.T) {
	e := newVerifyTestEnv(t)
	u := profileUser()
	u.EmailVerified = true
	u.PendingEmail = "new@goget.pw"
	expectResendUser(e, u)
	sent := expectIssueMail(e, u.PendingEmail)
	email, wait, err := e.uc.Resend(context.Background(), u.ID)
	require.NoError(t, err)
	require.Zero(t, wait)
	require.Equal(t, u.PendingEmail, email)
	waitVerifyMail(t, sent)
}

func TestResend_AlreadyVerified(t *testing.T) {
	e := newVerifyTestEnv(t)
	u := profileUser()
	u.EmailVerified = true
	expectResendUser(e, u)
	_, _, err := e.uc.Resend(context.Background(), u.ID)
	require.ErrorIs(t, err, domain.ErrEmailAlreadyVerified)
}

func TestResendByToken(t *testing.T) {
	e := newVerifyTestEnv(t)
	u := profileUser()
	v := verification(u.Email)
	v.ExpiresAt = time.Now().Add(-time.Hour)
	e.repo.EXPECT().Get(gomock.Any(), hashToken("old")).Return(v, nil)
	e.users.EXPECT().GetByID(gomock.Any(), u.ID).Return(u, nil)
	expectResendUser(e, u)
	sent := expectIssueMail(e, u.Email)
	email, wait, err := e.uc.ResendByToken(context.Background(), "old")
	require.NoError(t, err)
	require.Zero(t, wait)
	require.Equal(t, u.Email, email)
	waitVerifyMail(t, sent)
	e.repo.EXPECT().Get(gomock.Any(), hashToken("nope")).Return(domain.EmailVerification{}, domain.ErrInvalidVerifyToken)
	_, _, err = e.uc.ResendByToken(context.Background(), "nope")
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	u.Email = "new@goget.pw"
	e.repo.EXPECT().Get(gomock.Any(), hashToken("old")).Return(v, nil)
	e.users.EXPECT().GetByID(gomock.Any(), u.ID).Return(u, nil)
	_, _, err = e.uc.ResendByToken(context.Background(), "old")
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
}

func TestRegister_SendsVerificationLink(t *testing.T) {
	e := newTestUsecase(t)
	u := profileUser()
	sent := false
	expectTransaction(e.tx)
	e.users.EXPECT().Create(gomock.Any(), u.Email, gomock.Any(), u.FirstName, u.Phone).Return(u, nil)
	e.sessions.EXPECT().Create(gomock.Any(), gomock.Any(), u.ID, gomock.Any()).Return(nil)
	e.verifier.EXPECT().IssueLink(gomock.Any(), u.ID, u.Email).Return(func() { sent = true }, nil)
	user, _, err := e.Register(context.Background(), "Anna@GoGet.pw", "Secret123", u.FirstName, u.Phone)
	require.NoError(t, err)
	require.False(t, user.EmailVerified)
	require.True(t, sent)
}

func TestUpdateProfile_UnverifiedEmailIsReplaced(t *testing.T) {
	e := newProfileTestEnv(t)
	expectProfileBefore(e, profileUser())
	sent := false
	e.verifier.EXPECT().IssueLink(gomock.Any(), e.userID, "new@goget.pw").Return(func() { sent = true }, nil)
	e.resets.EXPECT().UseAll(gomock.Any(), e.userID).Return(nil)
	u := profileUser()
	u.Email = "new@goget.pw"
	expectProfileResult(e, domain.ProfileUpdate{Email: ptr(u.Email)}, domain.Profile{User: u})
	p, err := e.uc.UpdateProfile(context.Background(), e.userID, domain.ProfileUpdate{Email: ptr(u.Email)})
	require.NoError(t, err)
	require.Equal(t, u, p.User)
	require.True(t, sent)
}

func expectPendingChange(e *profileTestEnv, before domain.User, email string) {
	expectProfileBefore(e, before)
	e.users.EXPECT().GetByEmail(gomock.Any(), email).Return(domain.User{}, domain.ErrUserNotFound)
	e.verifier.EXPECT().IssueLink(gomock.Any(), e.userID, email).Return(func() {}, nil)
	e.users.EXPECT().SetPendingEmail(gomock.Any(), e.userID, ptr(email)).Return(nil)
	result := before
	result.PendingEmail = email
	expectProfileResult(e, domain.ProfileUpdate{}, domain.Profile{User: result})
}

func TestUpdateProfile_VerifiedEmailWaitsForConfirmation(t *testing.T) {
	e := newProfileTestEnv(t)
	u := profileUser()
	u.EmailVerified = true
	expectPendingChange(e, u, "new@goget.pw")
	p, err := e.uc.UpdateProfile(context.Background(), e.userID, domain.ProfileUpdate{Email: ptr("new@goget.pw")})
	require.NoError(t, err)
	require.Equal(t, u.Email, p.Email)
	require.Equal(t, "new@goget.pw", p.PendingEmail)
	u = p.User
	expectProfileBefore(e, u)
	expectProfileResult(e, domain.ProfileUpdate{}, p)
	_, err = e.uc.UpdateProfile(context.Background(), e.userID, domain.ProfileUpdate{Email: ptr(u.PendingEmail)})
	require.NoError(t, err)
	expectProfileBefore(e, u)
	e.users.EXPECT().SetPendingEmail(gomock.Any(), e.userID, nil).Return(nil)
	e.verifier.EXPECT().RevokeLinks(gomock.Any(), e.userID).Return(nil)
	u.PendingEmail = ""
	expectProfileResult(e, domain.ProfileUpdate{}, domain.Profile{User: u})
	p, err = e.uc.UpdateProfile(context.Background(), e.userID, domain.ProfileUpdate{Email: ptr(u.Email)})
	require.NoError(t, err)
	require.Empty(t, p.PendingEmail)
}

func TestUpdateProfile_PendingEmailMustBeFree(t *testing.T) {
	e := newProfileTestEnv(t)
	u := profileUser()
	u.EmailVerified = true
	expectProfileBefore(e, u)
	e.users.EXPECT().GetByEmail(gomock.Any(), "boris@goget.pw").Return(domain.User{ID: 8}, nil)
	_, err := e.uc.UpdateProfile(context.Background(), e.userID, domain.ProfileUpdate{Email: ptr("boris@goget.pw")})
	require.ErrorIs(t, err, domain.ErrEmailTaken)
}

func TestResetPassword_ConfirmsEmail(t *testing.T) {
	e := newResetTestEnv(t)
	expectPasswordReset(e, "token", "NewPass123")
	require.NoError(t, e.uc.ResetPassword(context.Background(), "token", "NewPass123"))
}

func TestResend_RevokesPreviousLink(t *testing.T) {
	e := newVerifyTestEnv(t)
	u := profileUser()
	expectResendUser(e, u)
	sent := expectIssueMail(e, u.Email)
	_, _, err := e.uc.Resend(context.Background(), u.ID)
	require.NoError(t, err)
	fresh := waitVerifyMail(t, sent)
	old := verification(u.Email)
	now := time.Now()
	old.UsedAt = &now
	expectVerify(e, "old", old, u)
	_, err = e.uc.Verify(context.Background(), "old")
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	expectVerify(e, fresh, verification(u.Email), u)
	e.users.EXPECT().SetEmailVerified(gomock.Any(), u.ID).Return(nil)
	e.repo.EXPECT().UseAll(gomock.Any(), u.ID).Return(nil)
	_, err = e.uc.Verify(context.Background(), fresh)
	require.NoError(t, err)
}

func TestUpdateProfile_CancelledChangeLinkStaysDead(t *testing.T) {
	e := newProfileTestEnv(t)
	u := profileUser()
	u.EmailVerified = true
	u.PendingEmail = "new@goget.pw"
	expectProfileBefore(e, u)
	e.users.EXPECT().SetPendingEmail(gomock.Any(), u.ID, nil).Return(nil)
	e.verifier.EXPECT().RevokeLinks(gomock.Any(), u.ID).Return(nil)
	cleared := u
	cleared.PendingEmail = ""
	expectProfileResult(e, domain.ProfileUpdate{}, domain.Profile{User: cleared})
	_, err := e.uc.UpdateProfile(context.Background(), u.ID, domain.ProfileUpdate{Email: ptr(u.Email)})
	require.NoError(t, err)
	expectPendingChange(e, cleared, u.PendingEmail)
	_, err = e.uc.UpdateProfile(context.Background(), u.ID, domain.ProfileUpdate{Email: ptr(u.PendingEmail)})
	require.NoError(t, err)
	v := newVerifyTestEnv(t)
	old := verification(u.PendingEmail)
	now := time.Now()
	old.UsedAt = &now
	expectVerify(v, "old", old, u)
	_, err = v.uc.Verify(context.Background(), "old")
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	expectVerify(v, "fresh", verification(u.PendingEmail), u)
	v.users.EXPECT().ConfirmPendingEmail(gomock.Any(), u.ID).Return(nil)
	v.resets.EXPECT().UseAll(gomock.Any(), u.ID).Return(nil)
	v.repo.EXPECT().UseAll(gomock.Any(), u.ID).Return(nil)
	res, err := v.uc.Verify(context.Background(), "fresh")
	require.NoError(t, err)
	require.True(t, res.Changed)
}

func TestUpdateProfile_ReplacedPendingEmailRevokesLink(t *testing.T) {
	e := newProfileTestEnv(t)
	u := profileUser()
	u.EmailVerified = true
	u.PendingEmail = "first@goget.pw"
	expectPendingChange(e, u, "second@goget.pw")
	p, err := e.uc.UpdateProfile(context.Background(), u.ID, domain.ProfileUpdate{Email: ptr("second@goget.pw")})
	require.NoError(t, err)
	require.Equal(t, "second@goget.pw", p.PendingEmail)
	v := newVerifyTestEnv(t)
	expectVerify(v, "first", verification(u.PendingEmail), p.User)
	_, err = v.uc.Verify(context.Background(), "first")
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
}

func TestUpdateProfile_EmailChangeRespectsCooldown(t *testing.T) {
	e := newProfileTestEnv(t)
	u := profileUser()
	u.EmailVerified = true
	u.PendingEmail = "first@goget.pw"
	expectProfileBefore(e, u)
	e.users.EXPECT().GetByEmail(gomock.Any(), "second@goget.pw").Return(domain.User{}, domain.ErrUserNotFound)
	e.verifier.EXPECT().IssueLink(gomock.Any(), u.ID, "second@goget.pw").Return(nil, &domain.CooldownError{Wait: 55 * time.Second})
	_, err := e.uc.UpdateProfile(context.Background(), u.ID, domain.ProfileUpdate{Email: ptr("second@goget.pw")})
	var cooldown *domain.CooldownError
	require.ErrorAs(t, err, &cooldown)
	require.Greater(t, cooldown.Wait, 50*time.Second)
}

func TestResetPassword_RevokesEmailChange(t *testing.T) {
	e := newResetTestEnv(t)
	expectPasswordReset(e, "reset", "NewSecret1")
	require.NoError(t, e.uc.ResetPassword(context.Background(), "reset", "NewSecret1"))
	v := newVerifyTestEnv(t)
	u := profileUser()
	u.EmailVerified = true
	old := verification("attacker@goget.pw")
	now := time.Now()
	old.UsedAt = &now
	expectVerify(v, "old", old, u)
	_, err := v.uc.Verify(context.Background(), "old")
	require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
}
