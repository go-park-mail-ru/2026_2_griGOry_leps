package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/csrf"
)

func newCSRFHandler() *CSRFHandler {
	return NewCSRFHandler(csrf.New([]byte("test-secret"), time.Hour), false)
}

func fetchCSRFToken(t *testing.T, h *CSRFHandler, cookies ...*http.Cookie) (string, *httptest.ResponseRecorder) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/csrf", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.Token(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["csrf_token"])
	return resp["csrf_token"], rec
}

func responseCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func serveProtected(h *CSRFHandler, method, token string, cookies ...*http.Cookie) int {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(method, "/api/logout", nil)
	if token != "" {
		req.Header.Set(csrfHeader, token)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.Middleware(next).ServeHTTP(rec, req)
	return rec.Code
}

func TestCSRF_Token_SetsAnonCookie(t *testing.T) {
	_, rec := fetchCSRFToken(t, newCSRFHandler())

	cookie := responseCookie(rec, csrfCookieName)
	require.NotNil(t, cookie)
	require.Len(t, cookie.Value, 64)
	require.True(t, cookie.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	require.Equal(t, "/", cookie.Path)
}

func TestCSRF_Token_NotCached(t *testing.T) {
	h := newCSRFHandler()

	for _, cookies := range [][]*http.Cookie{
		nil,
		{{Name: sessionCookieName, Value: "session-a"}},
	} {
		_, rec := fetchCSRFToken(t, h, cookies...)

		require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	}
}

func TestCSRF_Token_ReusesAnonCookie(t *testing.T) {
	h := newCSRFHandler()
	anon := &http.Cookie{Name: csrfCookieName, Value: "existing"}

	token, rec := fetchCSRFToken(t, h, anon)

	require.Nil(t, responseCookie(rec, csrfCookieName))
	require.Equal(t, http.StatusOK, serveProtected(h, http.MethodPost, token, anon))
}

func TestCSRF_Token_BoundToSession(t *testing.T) {
	h := newCSRFHandler()
	session := &http.Cookie{Name: sessionCookieName, Value: "session-a"}

	token, rec := fetchCSRFToken(t, h, session)

	require.Nil(t, responseCookie(rec, csrfCookieName))
	require.Equal(t, http.StatusOK, serveProtected(h, http.MethodPost, token, session))
}

func TestCSRF_Middleware_SafeMethodsPass(t *testing.T) {
	h := newCSRFHandler()

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		require.Equal(t, http.StatusOK, serveProtected(h, method, ""), method)
	}
}

func TestCSRF_Middleware_MissingToken(t *testing.T) {
	captureLogs(t)
	h := newCSRFHandler()
	session := &http.Cookie{Name: sessionCookieName, Value: "session-a"}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		require.Equal(t, http.StatusForbidden, serveProtected(h, method, "", session), method)
	}
}

func TestCSRF_Middleware_OtherSession(t *testing.T) {
	captureLogs(t)
	h := newCSRFHandler()
	token, _ := fetchCSRFToken(t, h, &http.Cookie{Name: sessionCookieName, Value: "session-a"})

	code := serveProtected(h, http.MethodPost, token, &http.Cookie{Name: sessionCookieName, Value: "session-b"})

	require.Equal(t, http.StatusForbidden, code)
}

func TestCSRF_Middleware_AnonTokenRejectedAfterLogin(t *testing.T) {
	captureLogs(t)
	h := newCSRFHandler()
	token, rec := fetchCSRFToken(t, h)
	anon := responseCookie(rec, csrfCookieName)

	code := serveProtected(h, http.MethodPost, token, anon, &http.Cookie{Name: sessionCookieName, Value: "session-a"})

	require.Equal(t, http.StatusForbidden, code)
}

func TestCSRF_Middleware_NoCookies(t *testing.T) {
	buf := captureLogs(t)
	h := newCSRFHandler()
	token, _ := fetchCSRFToken(t, h)

	require.Equal(t, http.StatusForbidden, serveProtected(h, http.MethodPost, token))

	var logged bool
	for _, record := range logRecords(t, buf) {
		if record["msg"] == "csrf check failed" {
			logged = true
			require.Equal(t, "WARN", record["level"])
		}
	}
	require.True(t, logged, buf.String())
}
