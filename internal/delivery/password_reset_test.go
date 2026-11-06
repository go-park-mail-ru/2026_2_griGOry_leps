package delivery

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestForgotPassword(t *testing.T) {
	mock := NewMockPasswordResetUsecase(gomock.NewController(t))
	mock.EXPECT().RequestReset(gomock.Any(), "+79990000001").Return(nil)
	h := NewPasswordResetHandler(mock)
	rec := httptest.NewRecorder()

	h.Forgot(rec, httptest.NewRequest(http.MethodPost, "/api/password/forgot", strings.NewReader(`{"login":"+79990000001"}`)))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
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

			mock := NewMockPasswordResetUsecase(gomock.NewController(t))
			if tt.name != "bad json" {
				login := "x"
				if tt.name == "internal" {
					login = "a@b.ru"
				}
				mock.EXPECT().RequestReset(gomock.Any(), login).Return(tt.err)
			}
			NewPasswordResetHandler(mock).Forgot(rec, httptest.NewRequest(http.MethodPost, "/api/password/forgot", strings.NewReader(tt.body)))

			require.Equal(t, tt.status, rec.Code)
			require.JSONEq(t, tt.resp, rec.Body.String())
		})
	}
}

func TestCheckResetToken(t *testing.T) {
	mock := NewMockPasswordResetUsecase(gomock.NewController(t))
	mock.EXPECT().CheckToken(gomock.Any(), "good").Return("anna@goget.pw", nil)
	mock.EXPECT().CheckToken(gomock.Any(), "bad").Return("", domain.ErrInvalidResetToken)
	h := NewPasswordResetHandler(mock)

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
	mock := NewMockPasswordResetUsecase(gomock.NewController(t))
	mock.EXPECT().ResetPassword(gomock.Any(), "abc", "NewSecret1").Return(nil)
	h := NewPasswordResetHandler(mock)
	rec := httptest.NewRecorder()

	h.Reset(rec, httptest.NewRequest(http.MethodPost, "/api/password/reset", strings.NewReader(`{"token":"abc","password":"NewSecret1"}`)))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
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

			mock := NewMockPasswordResetUsecase(gomock.NewController(t))
			mock.EXPECT().ResetPassword(gomock.Any(), "", "").Return(tt.err)
			NewPasswordResetHandler(mock).Reset(rec, httptest.NewRequest(http.MethodPost, "/api/password/reset", strings.NewReader(`{}`)))

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Equal(t, tt.field, decodeBody(t, rec)["field"])
		})
	}
}
