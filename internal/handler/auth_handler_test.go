package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
)

func newAuthHandler() *AuthHandler {
	authUC := usecase.NewAuthUsecase(repository.NewUserRepository(), repository.NewSessionRepository())
	return NewAuthHandler(authUC, false)
}

func TestAuthHandler_Register_BadJSON(t *testing.T) {
	h := newAuthHandler()
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthHandler_Register_Success(t *testing.T) {
	h := newAuthHandler()
	body := `{"email":"a@b.ru","password":"Secret123","nickname":"ivan","phone":"+79001234567"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	cookie := rec.Header().Get("Set-Cookie")
	require.Contains(t, cookie, "session_id=")
	require.Contains(t, cookie, "HttpOnly")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "ivan", resp["nickname"])
	require.Equal(t, "a@b.ru", resp["email"])
}

func TestAuthHandler_Register_ValidationError(t *testing.T) {
	h := newAuthHandler()
	body := `{"email":"a@b.ru","password":"short","nickname":"ivan","phone":"+79001234567"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "password")
}

func TestAuthHandler_Register_EmailTaken(t *testing.T) {
	h := newAuthHandler()
	body := `{"email":"a@b.ru","password":"Secret123","nickname":"ivan","phone":"+79001234567"}`

	req1 := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	h.Register(httptest.NewRecorder(), req1)

	req2 := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	rec2 := httptest.NewRecorder()
	h.Register(rec2, req2)

	require.Equal(t, http.StatusConflict, rec2.Code)
}

func TestAuthHandler_Login_BadJSON(t *testing.T) {
	h := newAuthHandler()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("nope"))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthHandler_Login_Success(t *testing.T) {
	h := newAuthHandler()
	registerBody := `{"email":"a@b.ru","password":"Secret123","nickname":"ivan","phone":"+79001234567"}`
	h.Register(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(registerBody)))

	loginBody := `{"login":"a@b.ru","password":"Secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(loginBody))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	cookie := rec.Header().Get("Set-Cookie")
	require.Contains(t, cookie, "session_id=")
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	h := newAuthHandler()
	body := `{"login":"a@b.ru","password":"Secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthHandler_Logout_NoCookie(t *testing.T) {
	h := newAuthHandler()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	cookie := rec.Header().Get("Set-Cookie")
	require.Contains(t, cookie, "session_id=")
}

func TestAuthHandler_Logout_WithCookie(t *testing.T) {
	h := newAuthHandler()

	registerBody := `{"email":"a@b.ru","password":"Secret123","nickname":"ivan","phone":"+79001234567"}`
	regRec := httptest.NewRecorder()
	h.Register(regRec, httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(registerBody)))

	resp := regRec.Result()
	cookies := resp.Cookies()
	require.NotEmpty(t, cookies)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(cookies[0])
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthHandler_Me_NoCookie(t *testing.T) {
	h := newAuthHandler()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthHandler_Me_Success(t *testing.T) {
	h := newAuthHandler()

	registerBody := `{"email":"a@b.ru","password":"Secret123","nickname":"ivan","phone":"+79001234567"}`
	regRec := httptest.NewRecorder()
	h.Register(regRec, httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(registerBody)))

	cookies := regRec.Result().Cookies()
	require.NotEmpty(t, cookies)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(cookies[0])
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "ivan", resp["nickname"])
}

func TestAuthHandler_Me_InvalidSession(t *testing.T) {
	h := newAuthHandler()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "invalid"})
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthHandler_UsecaseWiring(t *testing.T) {
	authUC := usecase.NewAuthUsecase(repository.NewUserRepository(), repository.NewSessionRepository())
	h := NewAuthHandler(authUC, false)

	body := `{"email":"x@y.ru","password":"Secret123","nickname":"xxx","phone":"+79005555555"}`
	rec := httptest.NewRecorder()
	h.Register(rec, httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body)))

	require.Equal(t, http.StatusCreated, rec.Code)
	ctx := context.Background()
	user, _, err := authUC.Login(ctx, "x@y.ru", "Secret123")
	require.NoError(t, err)
	require.Equal(t, "xxx", user.Nickname)
}
