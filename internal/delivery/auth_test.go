package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type authStub struct {
	err      error
	register func(context.Context, string, string, string, string) (domain.User, domain.Session, error)
}

func (s authStub) Register(ctx context.Context, email, password, firstName, phone string) (domain.User, domain.Session, error) {
	if s.register != nil {
		return s.register(ctx, email, password, firstName, phone)
	}
	return domain.User{ID: 1, Email: email, FirstName: firstName, Phone: phone},
		domain.Session{ID: "test-session", UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}, s.err
}

func (s authStub) Login(ctx context.Context, login, password string) (domain.User, domain.Session, error) {
	return domain.User{ID: 1, Email: login},
		domain.Session{ID: "test-session", UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}, s.err
}

func (s authStub) Logout(ctx context.Context, id string) error {
	return s.err
}

func (s authStub) Me(ctx context.Context, id string) (domain.User, error) {
	return domain.User{ID: 1, FirstName: "Иван"}, s.err
}

func newAuthHandler() *AuthHandler { return NewAuthHandler(authStub{}, false) }

func TestAuth_Register_PassesArguments(t *testing.T) {
	called := false
	h := NewAuthHandler(authStub{register: func(ctx context.Context, email, password, name, phone string) (domain.User, domain.Session, error) {
		called = true
		require.Equal(t, "x@y.ru", email)
		require.Equal(t, "Secret123", password)
		require.Equal(t, "Ян", name)
		require.Equal(t, "+79005555555", phone)
		return domain.User{FirstName: name}, domain.Session{}, nil
	}}, false)

	body := `{"email":"x@y.ru","password":"Secret123","first_name":"Ян","phone":"+79005555555"}`
	rec := httptest.NewRecorder()
	h.Register(rec, httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body)))

	require.True(t, called)
	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestAuth_Register_BadJSON(t *testing.T) {
	h := newAuthHandler()
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuth_Register_Success(t *testing.T) {
	h := newAuthHandler()
	body := `{"email":"a@b.ru","password":"Secret123","first_name":"Иван","phone":"+79001234567"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	cookie := rec.Header().Get("Set-Cookie")
	require.Contains(t, cookie, "session_id=")
	require.Contains(t, cookie, "HttpOnly")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "Иван", resp["first_name"])
	require.Equal(t, "a@b.ru", resp["email"])
}

func TestAuth_Register_ValidationError(t *testing.T) {
	h := NewAuthHandler(authStub{err: domain.ErrWeakPassword}, false)
	body := `{"email":"a@b.ru","password":"short","first_name":"Иван","phone":"+79001234567"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "password")
}

func TestAuth_Register_EmailTaken(t *testing.T) {
	h := NewAuthHandler(authStub{err: domain.ErrEmailTaken}, false)
	body := `{"email":"a@b.ru","password":"Secret123","first_name":"Иван","phone":"+79001234567"}`

	req1 := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	h.Register(httptest.NewRecorder(), req1)

	req2 := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	rec2 := httptest.NewRecorder()
	h.Register(rec2, req2)

	require.Equal(t, http.StatusConflict, rec2.Code)
}

func TestAuth_Register_Login_BadJSON(t *testing.T) {
	h := newAuthHandler()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("nope"))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuth_Login_Success(t *testing.T) {
	h := newAuthHandler()
	registerBody := `{"email":"a@b.ru","password":"Secret123","first_name":"Иван","phone":"+79001234567"}`
	h.Register(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(registerBody)))

	loginBody := `{"login":"a@b.ru","password":"Secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(loginBody))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	cookie := rec.Header().Get("Set-Cookie")
	require.Contains(t, cookie, "session_id=")
}

func TestAuth_Login_InvalidCredentials(t *testing.T) {
	h := NewAuthHandler(authStub{err: domain.ErrInvalidLogin}, false)
	body := `{"login":"a@b.ru","password":"Secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_Logout_NoCookie(t *testing.T) {
	h := newAuthHandler()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	cookie := rec.Header().Get("Set-Cookie")
	require.Contains(t, cookie, "session_id=")
}

func TestAuth_Logout_WithCookie(t *testing.T) {
	h := newAuthHandler()

	registerBody := `{"email":"a@b.ru","password":"Secret123","first_name":"Иван","phone":"+79001234567"}`
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

func TestAuth_Me_NoCookie(t *testing.T) {
	h := newAuthHandler()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_Me_Success(t *testing.T) {
	h := newAuthHandler()

	registerBody := `{"email":"a@b.ru","password":"Secret123","first_name":"Иван","phone":"+79001234567"}`
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
	require.Equal(t, "Иван", resp["first_name"])
}

func TestAuth_Me_InvalidSession(t *testing.T) {
	h := NewAuthHandler(authStub{err: domain.ErrSessionExpired}, false)
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "invalid"})
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
