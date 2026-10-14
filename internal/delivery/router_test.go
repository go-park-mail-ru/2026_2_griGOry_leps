package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type panicAdUsecase struct{}

func (panicAdUsecase) List(context.Context, domain.ListAdsParams) (domain.AdPage, error) {
	panic("boom")
}

func newTestRouter() http.Handler {
	return NewRouter("http://front.test", newAuthHandler(), NewAdHandler(panicAdUsecase{}))
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
