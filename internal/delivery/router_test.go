package delivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type panicAdUsecase struct{}

func (panicAdUsecase) List(context.Context, domain.ListAdsParams) (domain.AdPage, error) {
	panic("boom")
}

func newTestRouter() http.Handler {
	return NewRouter("http://front.test", newAuthHandler(), NewAdHandler(panicAdUsecase{}), newCSRFHandler())
}

func TestRouter_SetsRequestIDHeader(t *testing.T) {
	buf := captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	id := rec.Header().Get(requestIDHeader)
	require.Regexp(t, generatedID, id)
	require.Equal(t, id, accessRecord(t, buf)["request_id"])
}

func TestRouter_PreflightHasRequestID(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/api/login", nil))

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.NotEmpty(t, rec.Header().Get(requestIDHeader))
}

func TestRouter_CORSAllowsAndExposesRequestID(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), requestIDHeader)
	require.Equal(t, requestIDHeader, rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestRouter_PanicLoggedWith500(t *testing.T) {
	buf := captureLogs(t)
	req := httptest.NewRequest(http.MethodGet, "/api/listings", nil)
	req.Header.Set(requestIDHeader, "panic-req")
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var panicLogged bool
	for _, record := range logRecords(t, buf) {
		if record["msg"] == "panic recovered" {
			panicLogged = true
			require.Equal(t, "panic-req", record["request_id"])
			require.Equal(t, "boom", record["panic"])
		}
	}
	require.True(t, panicLogged, buf.String())

	access := accessRecord(t, buf)
	require.EqualValues(t, http.StatusInternalServerError, access["status"])
	require.Equal(t, "panic-req", access["request_id"])
}

func TestRouter_LoginFlowWithCSRF(t *testing.T) {
	captureLogs(t)
	router := newTestRouter()

	tokenRec := httptest.NewRecorder()
	router.ServeHTTP(tokenRec, httptest.NewRequest(http.MethodGet, "/api/csrf", nil))
	require.Equal(t, http.StatusOK, tokenRec.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(tokenRec.Body.Bytes(), &resp))

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"login":"a@b.ru","password":"Secret123"}`))
	req.Header.Set(csrfHeader, resp["csrf_token"])
	for _, c := range tokenRec.Result().Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRouter_PostWithoutCSRFTokenForbidden(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/logout", nil))

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"error":"invalid csrf token"}`, rec.Body.String())
}

func TestRouter_SecurityHeaders(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), csrfHeader)
}
