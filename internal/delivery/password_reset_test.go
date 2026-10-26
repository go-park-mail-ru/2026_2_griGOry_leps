package delivery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type resetStub struct {
	err     error
	request func(login string)
	reset   func(token, password string)
}

func (s resetStub) RequestReset(_ context.Context, login string) error {
	if s.request != nil {
		s.request(login)
	}
	return s.err
}

func (s resetStub) CheckToken(_ context.Context, token string) (string, error) {
	if token != "good" {
		return "", domain.ErrInvalidResetToken
	}
	return "anna@goget.pw", s.err
}

func (s resetStub) ResetPassword(_ context.Context, token, password string) error {
	if s.reset != nil {
		s.reset(token, password)
	}
	return s.err
}

func TestForgotPassword(t *testing.T) {
	var got string
	h := NewPasswordResetHandler(resetStub{request: func(login string) { got = login }})
	rec := httptest.NewRecorder()

	h.Forgot(rec, httptest.NewRequest(http.MethodPost, "/api/password/forgot", strings.NewReader(`{"login":"+79990000001"}`)))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
	require.Equal(t, "+79990000001", got)
}

func TestForgotPassword_Errors(t *testing.T) {
	captureLogs(t)
	tests := []struct {
		name   string
		body   string
		err    error
		status int
		resp   string
	}{
		{"bad json", "{", nil, http.StatusBadRequest, `{"error":"invalid request body"}`},
		{"bad login", `{"login":"x"}`, domain.ErrInvalidContact, http.StatusBadRequest, `{"error":"invalid email or phone","field":"login"}`},
		{"internal", `{"login":"a@b.ru"}`, errors.New("db down"), http.StatusInternalServerError, `{"error":"internal error"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			NewPasswordResetHandler(resetStub{err: tt.err}).Forgot(rec, httptest.NewRequest(http.MethodPost, "/api/password/forgot", strings.NewReader(tt.body)))

			require.Equal(t, tt.status, rec.Code)
			require.JSONEq(t, tt.resp, rec.Body.String())
		})
	}
}

func TestCheckResetToken(t *testing.T) {
	h := NewPasswordResetHandler(resetStub{})

	rec := httptest.NewRecorder()
	h.Check(rec, httptest.NewRequest(http.MethodGet, "/api/password/reset?token=good", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"email":"anna@goget.pw"}`, rec.Body.String())

	rec = httptest.NewRecorder()
	h.Check(rec, httptest.NewRequest(http.MethodGet, "/api/password/reset?token=bad", nil))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"invalid or expired token","field":"token"}`, rec.Body.String())
}

func TestResetPassword(t *testing.T) {
	var gotToken, gotPassword string
	h := NewPasswordResetHandler(resetStub{reset: func(token, password string) { gotToken, gotPassword = token, password }})
	rec := httptest.NewRecorder()

	h.Reset(rec, httptest.NewRequest(http.MethodPost, "/api/password/reset", strings.NewReader(`{"token":"abc","password":"NewSecret1"}`)))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
	require.Equal(t, "abc", gotToken)
	require.Equal(t, "NewSecret1", gotPassword)
}

func TestResetPassword_Errors(t *testing.T) {
	tests := []struct {
		err   error
		field string
	}{
		{domain.ErrInvalidResetToken, "token"},
		{domain.ErrWeakPassword, "password"},
		{domain.ErrPasswordTooLong, "password"},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			rec := httptest.NewRecorder()

			NewPasswordResetHandler(resetStub{err: tt.err}).Reset(rec, httptest.NewRequest(http.MethodPost, "/api/password/reset", strings.NewReader(`{}`)))

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Equal(t, tt.field, decodeBody(t, rec)["field"])
		})
	}
}
