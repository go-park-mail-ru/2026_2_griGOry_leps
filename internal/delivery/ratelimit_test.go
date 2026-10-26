package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/ratelimit"
)

func postLogin(t *testing.T, router http.Handler, ip string) *httptest.ResponseRecorder {
	t.Helper()

	tokenRec := httptest.NewRecorder()
	router.ServeHTTP(tokenRec, httptest.NewRequest(http.MethodGet, "/api/csrf", nil))
	var resp map[string]string
	require.NoError(t, json.Unmarshal(tokenRec.Body.Bytes(), &resp))

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"login":"a@b.ru","password":"Secret123"}`))
	req.RemoteAddr = ip + ":40000"
	req.Header.Set(csrfHeader, resp["csrf_token"])
	for _, c := range tokenRec.Result().Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestRouter_LoginRateLimited(t *testing.T) {
	captureLogs(t)
	router := newTestRouter()

	for i := 0; i < 10; i++ {
		require.Equal(t, http.StatusOK, postLogin(t, router, "10.0.0.1").Code)
	}

	rec := postLogin(t, router, "10.0.0.1")
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.JSONEq(t, `{"error":"too many requests"}`, rec.Body.String())

	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.InDelta(t, 60, retryAfter, 1)

	require.Equal(t, http.StatusOK, postLogin(t, router, "10.0.0.2").Code)
}

func TestRateLimit_RetryAfterAtLeastOneSecond(t *testing.T) {
	captureLogs(t)
	l := ratelimit.New(1, 300*time.Millisecond)
	h := rateLimit(l, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	h(httptest.NewRecorder(), req)
	rec := httptest.NewRecorder()
	h(rec, req)

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "1", rec.Header().Get("Retry-After"))
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	req.RemoteAddr = "203.0.113.5:51234"
	require.Equal(t, "203.0.113.5", clientIP(req))

	req.RemoteAddr = "[2001:db8::1]:443"
	require.Equal(t, "2001:db8::1", clientIP(req))
}
