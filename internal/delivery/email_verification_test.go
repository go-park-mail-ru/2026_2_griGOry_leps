package delivery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type verifyStub struct {
	token     string
	userID    int32
	result    domain.EmailVerifyResult
	wait      time.Duration
	err       error
	cancelled bool
}

func (s *verifyStub) Verify(_ context.Context, token string) (domain.EmailVerifyResult, error) {
	s.token = token
	return s.result, s.err
}

func (s *verifyStub) Resend(_ context.Context, userID int32) (string, time.Duration, error) {
	s.userID = userID
	return "anna@goget.pw", s.wait, s.err
}

func (s *verifyStub) ResendByToken(_ context.Context, token string) (string, time.Duration, error) {
	s.token = token
	return "anna@goget.pw", s.wait, s.err
}

func (s *verifyStub) CancelEmailChange(_ context.Context, userID int32) error {
	s.userID, s.cancelled = userID, true
	return s.err
}

func postToken(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/api/email/verify", strings.NewReader(body))
}

func TestVerifyEmail(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stub   *verifyStub
		status int
		body   string
	}{
		{"ok", &verifyStub{result: domain.EmailVerifyResult{Email: "anna@goget.pw"}}, http.StatusOK,
			`{"email":"anna@goget.pw","changed":false}`},
		{"changed", &verifyStub{result: domain.EmailVerifyResult{Email: "new@goget.pw", Changed: true}}, http.StatusOK,
			`{"email":"new@goget.pw","changed":true}`},
		{"expired", &verifyStub{result: domain.EmailVerifyResult{Email: "anna@goget.pw"}, err: domain.ErrInvalidVerifyToken},
			http.StatusBadRequest, `{"error":"invalid or expired token","field":"token","email":"anna@goget.pw"}`},
		{"unknown", &verifyStub{err: domain.ErrInvalidVerifyToken}, http.StatusBadRequest,
			`{"error":"invalid or expired token","field":"token"}`},
		{"already", &verifyStub{err: domain.ErrEmailAlreadyVerified}, http.StatusConflict, `{"error":"email already verified"}`},
		{"taken", &verifyStub{err: domain.ErrEmailTaken}, http.StatusConflict, `{"error":"email already registered","field":"email"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			NewEmailVerificationHandler(tc.stub).Verify(rec, postToken(`{"token":"abc"}`))

			require.Equal(t, tc.status, rec.Code)
			require.Equal(t, "abc", tc.stub.token)
			require.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestVerifyEmail_BadBody(t *testing.T) {
	rec := httptest.NewRecorder()

	NewEmailVerificationHandler(&verifyStub{}).Verify(rec, postToken(`{`))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResendVerification(t *testing.T) {
	for _, tc := range []struct {
		name       string
		stub       *verifyStub
		status     int
		retryAfter string
	}{
		{"sent", &verifyStub{}, http.StatusOK, ""},
		{"cooldown", &verifyStub{wait: 42 * time.Second}, http.StatusTooManyRequests, "42"},
		{"already verified", &verifyStub{err: domain.ErrEmailAlreadyVerified}, http.StatusConflict, ""},
		{"db down", &verifyStub{err: errors.New("db down")}, http.StatusInternalServerError, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			captureLogs(t)
			rec := httptest.NewRecorder()

			NewEmailVerificationHandler(tc.stub).Resend(rec, withAuth(httptest.NewRequest(http.MethodPost, "/api/email/verify/resend", nil)))

			require.Equal(t, tc.status, rec.Code)
			require.Equal(t, int32(7), tc.stub.userID)
			require.Equal(t, tc.retryAfter, rec.Header().Get("Retry-After"))
			if tc.status == http.StatusOK {
				require.JSONEq(t, `{"email":"anna@goget.pw"}`, rec.Body.String())
			}
		})
	}
}

func TestResendByToken(t *testing.T) {
	stub := &verifyStub{}
	rec := httptest.NewRecorder()
	NewEmailVerificationHandler(stub).ResendByToken(rec, postToken(`{"token":"old"}`))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "old", stub.token)

	rec = httptest.NewRecorder()
	NewEmailVerificationHandler(&verifyStub{err: domain.ErrInvalidVerifyToken}).ResendByToken(rec, postToken(`{"token":"x"}`))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"invalid or expired token","field":"token"}`, rec.Body.String())
}

func TestCancelEmailChange(t *testing.T) {
	stub := &verifyStub{}
	rec := httptest.NewRecorder()
	NewEmailVerificationHandler(stub).CancelEmailChange(rec, withAuth(httptest.NewRequest(http.MethodDelete, "/api/me/email/pending", nil)))
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, stub.cancelled)
	require.Equal(t, int32(7), stub.userID)

	rec = httptest.NewRecorder()
	NewEmailVerificationHandler(&verifyStub{err: domain.ErrNoPendingEmail}).
		CancelEmailChange(rec, withAuth(httptest.NewRequest(http.MethodDelete, "/api/me/email/pending", nil)))
	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestRequireVerifiedEmail(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	for _, tc := range []struct {
		verified bool
		status   int
	}{
		{true, http.StatusNoContent},
		{false, http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/me/addresses", nil)
		req = req.WithContext(context.WithValue(req.Context(), emailVerifiedKey{}, tc.verified))
		rec := httptest.NewRecorder()

		requireVerifiedEmail(next).ServeHTTP(rec, req)

		require.Equal(t, tc.status, rec.Code)
		if !tc.verified {
			require.JSONEq(t, `{"error":"email not verified"}`, rec.Body.String())
		}
	}
}

func TestRequireAuth_PutsEmailVerifiedIntoContext(t *testing.T) {
	var verified, seen bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		verified, seen = r.Context().Value(emailVerifiedKey{}).(bool)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "s"})

	NewAuthHandler(authStub{verified: true}, false).RequireAuth(next).ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, seen)
	require.True(t, verified)
}
