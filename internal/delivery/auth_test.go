package delivery

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=usecase.go -destination=usecase_mock_test.go -package=delivery

import (
	"bytes"
	"encoding/json"
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

func TestAuth_Register_PassesArguments(t *testing.T) {
	mock := NewMockAuthUsecase(gomock.NewController(t))
	mock.EXPECT().Register(gomock.Any(), "x@y.ru", "Secret123", "Ян", "+79005555555").Return(domain.User{FirstName: "Ян"}, domain.Session{}, nil)
	h := NewAuthHandler(mock, false)
	body := `{"email":"x@y.ru","password":"Secret123","first_name":"Ян","phone":"+79005555555"}`
	rec := httptest.NewRecorder()
	h.Register(rec, httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body)))
	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestAuth_Register_BadJSON(t *testing.T) {
	h := NewAuthHandler(NewMockAuthUsecase(gomock.NewController(t)), false)
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuth_Register_Success(t *testing.T) {
	mock := NewMockAuthUsecase(gomock.NewController(t))
	mock.EXPECT().Register(gomock.Any(), "a@b.ru", "Secret123", "Иван", "+79001234567").Return(domain.User{ID: 1, FirstName: "Иван", Email: "a@b.ru"}, testSession(), nil)
	h := NewAuthHandler(mock, false)
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
	mock := NewMockAuthUsecase(gomock.NewController(t))
	mock.EXPECT().Register(gomock.Any(), "a@b.ru", "short", "Иван", "+79001234567").Return(domain.User{}, domain.Session{}, domain.ErrWeakPassword)
	h := NewAuthHandler(mock, false)
	body := `{"email":"a@b.ru","password":"short","first_name":"Иван","phone":"+79001234567"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "password")
}

func TestAuth_Register_EmailTaken(t *testing.T) {
	mock := NewMockAuthUsecase(gomock.NewController(t))
	mock.EXPECT().Register(gomock.Any(), "a@b.ru", "Secret123", "Иван", "+79001234567").Return(domain.User{}, domain.Session{}, domain.ErrEmailTaken).Times(2)
	h := NewAuthHandler(mock, false)
	body := `{"email":"a@b.ru","password":"Secret123","first_name":"Иван","phone":"+79001234567"}`

	req1 := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	h.Register(httptest.NewRecorder(), req1)

	req2 := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	rec2 := httptest.NewRecorder()
	h.Register(rec2, req2)

	require.Equal(t, http.StatusConflict, rec2.Code)
}

func TestAuth_Register_Login_BadJSON(t *testing.T) {
	h := NewAuthHandler(NewMockAuthUsecase(gomock.NewController(t)), false)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("nope"))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuth_Login_Success(t *testing.T) {
	mock := NewMockAuthUsecase(gomock.NewController(t))
	mock.EXPECT().Register(gomock.Any(), "a@b.ru", "Secret123", "Иван", "+79001234567").Return(domain.User{ID: 1, FirstName: "Иван", Email: "a@b.ru"}, testSession(), nil)
	mock.EXPECT().Login(gomock.Any(), "a@b.ru", "Secret123").Return(domain.User{ID: 1, Email: "a@b.ru"}, testSession(), nil)
	h := NewAuthHandler(mock, false)
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
	mock := NewMockAuthUsecase(gomock.NewController(t))
	mock.EXPECT().Login(gomock.Any(), "a@b.ru", "Secret123").Return(domain.User{}, domain.Session{}, domain.ErrInvalidLogin)
	h := NewAuthHandler(mock, false)
	body := `{"login":"a@b.ru","password":"Secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_Logout_NoCookie(t *testing.T) {
	h := NewAuthHandler(NewMockAuthUsecase(gomock.NewController(t)), false)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	cookie := rec.Header().Get("Set-Cookie")
	require.Contains(t, cookie, "session_id=")
}

func TestAuth_Logout_WithCookie(t *testing.T) {
	mock := NewMockAuthUsecase(gomock.NewController(t))
	mock.EXPECT().Register(gomock.Any(), "a@b.ru", "Secret123", "Иван", "+79001234567").Return(domain.User{ID: 1, FirstName: "Иван", Email: "a@b.ru"}, testSession(), nil)
	mock.EXPECT().Logout(gomock.Any(), "test-session").Return(nil)
	h := NewAuthHandler(mock, false)

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

func requireAuthRequest(h *AuthHandler, cookie *http.Cookie) (*httptest.ResponseRecorder, int32) {
	var gotID int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = userIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.RequireAuth(next).ServeHTTP(rec, req)
	return rec, gotID
}

func TestRequireAuth_NoCookie(t *testing.T) {
	rec, _ := requireAuthRequest(NewAuthHandler(NewMockAuthUsecase(gomock.NewController(t)), false), nil)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireAuth_ExpiredSession(t *testing.T) {
	mock := NewMockAuthUsecase(gomock.NewController(t))
	mock.EXPECT().Me(gomock.Any(), "old").Return(domain.User{}, domain.ErrSessionExpired)
	h := NewAuthHandler(mock, false)

	rec, _ := requireAuthRequest(h, &http.Cookie{Name: sessionCookieName, Value: "old"})

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireAuth_InternalError(t *testing.T) {
	captureLogs(t)
	mock := NewMockAuthUsecase(gomock.NewController(t))
	mock.EXPECT().Me(gomock.Any(), "s").Return(domain.User{}, errors.New("db down"))
	h := NewAuthHandler(mock, false)

	rec, _ := requireAuthRequest(h, &http.Cookie{Name: sessionCookieName, Value: "s"})

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRequireAuth_PutsUserIDInContext(t *testing.T) {
	mock := NewMockAuthUsecase(gomock.NewController(t))
	mock.EXPECT().Me(gomock.Any(), "s").Return(domain.User{ID: 1}, nil)
	rec, id := requireAuthRequest(NewAuthHandler(mock, false), &http.Cookie{Name: sessionCookieName, Value: "s"})

	require.Equal(t, http.StatusOK, rec.Code)
	require.EqualValues(t, 1, id)
}

func testSession() domain.Session {
	return domain.Session{ID: "test-session", UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}
}
