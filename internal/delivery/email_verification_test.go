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
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func postToken(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/api/email/verify", strings.NewReader(body))
}

func TestVerifyEmail(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result domain.EmailVerifyResult
		err    error
		status int
		body   string
	}{
		{"ok", domain.EmailVerifyResult{Email: "anna@goget.pw"}, nil, http.StatusOK,
			`{"email":"anna@goget.pw","changed":false}`},
		{"changed", domain.EmailVerifyResult{Email: "new@goget.pw", Changed: true}, nil, http.StatusOK,
			`{"email":"new@goget.pw","changed":true}`},
		{"expired", domain.EmailVerifyResult{Email: "anna@goget.pw"}, domain.ErrInvalidVerifyToken,
			http.StatusBadRequest, `{"error":"invalid or expired token","field":"token","email":"anna@goget.pw"}`},
		{"unknown", domain.EmailVerifyResult{}, domain.ErrInvalidVerifyToken, http.StatusBadRequest,
			`{"error":"invalid or expired token","field":"token"}`},
		{"already", domain.EmailVerifyResult{}, domain.ErrEmailAlreadyVerified, http.StatusConflict, `{"error":"email already verified"}`},
		{"taken", domain.EmailVerifyResult{}, domain.ErrEmailTaken, http.StatusConflict, `{"error":"email already registered","field":"email"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			mock := NewMockEmailVerificationUsecase(gomock.NewController(t))
			mock.EXPECT().Verify(gomock.Any(), "abc").Return(tc.result, tc.err)
			NewEmailVerificationHandler(mock).Verify(rec, postToken(`{"token":"abc"}`))

			require.Equal(t, tc.status, rec.Code)
			require.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestVerifyEmail_BadBody(t *testing.T) {
	rec := httptest.NewRecorder()

	NewEmailVerificationHandler(NewMockEmailVerificationUsecase(gomock.NewController(t))).Verify(rec, postToken(`{`))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResendVerification(t *testing.T) {
	for _, tc := range []struct {
		name       string
		wait       time.Duration
		err        error
		status     int
		retryAfter string
	}{
		{"sent", 0, nil, http.StatusOK, ""},
		{"cooldown", 42 * time.Second, nil, http.StatusTooManyRequests, "42"},
		{"already verified", 0, domain.ErrEmailAlreadyVerified, http.StatusConflict, ""},
		{"db down", 0, errors.New("db down"), http.StatusInternalServerError, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			captureLogs(t)
			rec := httptest.NewRecorder()

			mock := NewMockEmailVerificationUsecase(gomock.NewController(t))
			mock.EXPECT().Resend(gomock.Any(), int32(7)).Return("anna@goget.pw", tc.wait, tc.err)
			NewEmailVerificationHandler(mock).Resend(rec, withAuth(httptest.NewRequest(http.MethodPost, "/api/email/verify/resend", nil)))

			require.Equal(t, tc.status, rec.Code)
			require.Equal(t, tc.retryAfter, rec.Header().Get("Retry-After"))
			if tc.status == http.StatusOK {
				require.JSONEq(t, `{"email":"anna@goget.pw"}`, rec.Body.String())
			}
		})
	}
}

func TestResendByToken(t *testing.T) {
	stub := NewMockEmailVerificationUsecase(gomock.NewController(t))
	stub.EXPECT().ResendByToken(gomock.Any(), "old").Return("anna@goget.pw", time.Duration(0), nil)
	rec := httptest.NewRecorder()
	NewEmailVerificationHandler(stub).ResendByToken(rec, postToken(`{"token":"old"}`))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	stub.EXPECT().ResendByToken(gomock.Any(), "x").Return("", time.Duration(0), domain.ErrInvalidVerifyToken)
	NewEmailVerificationHandler(stub).ResendByToken(rec, postToken(`{"token":"x"}`))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"invalid or expired token","field":"token"}`, rec.Body.String())
}

func TestCancelEmailChange(t *testing.T) {
	stub := NewMockEmailVerificationUsecase(gomock.NewController(t))
	stub.EXPECT().CancelEmailChange(gomock.Any(), int32(7)).Return(nil)
	rec := httptest.NewRecorder()
	NewEmailVerificationHandler(stub).CancelEmailChange(rec, withAuth(httptest.NewRequest(http.MethodDelete, "/api/me/email/pending", nil)))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	stub.EXPECT().CancelEmailChange(gomock.Any(), int32(7)).Return(domain.ErrNoPendingEmail)
	NewEmailVerificationHandler(stub).
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

	mock := NewMockAuthUsecase(gomock.NewController(t))
	mock.EXPECT().Me(gomock.Any(), "s").Return(domain.User{ID: 1, EmailVerified: true}, nil)
	NewAuthHandler(mock, false).RequireAuth(next).ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, seen)
	require.True(t, verified)
}
