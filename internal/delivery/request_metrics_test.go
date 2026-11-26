package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

func TestRequestMetricsUseRouteTemplates(t *testing.T) {
	const template = "/metric-test/{id:[0-9]+}"
	t.Cleanup(func() {
		requestCount.DeleteLabelValues(http.MethodGet, template, "200")
		requestCount.DeleteLabelValues(http.MethodGet, template, "503")
		requestDuration.DeleteLabelValues(http.MethodGet, template)
	})
	router := newAPIRouter()
	router.HandleFunc(template, func(w http.ResponseWriter, r *http.Request) {
		if mux.Vars(r)["id"] == "7629" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}).Methods(http.MethodGet)
	handler := accessLogMiddleware(corsMiddleware("", http.TimeoutHandler(router, time.Second, "timeout")))

	for _, tc := range []struct {
		method string
		path   string
		code   int
	}{
		{http.MethodGet, "/metric-test/7628?token=secret-fixture", http.StatusOK},
		{http.MethodGet, "/metric-test/7629", http.StatusServiceUnavailable},
		{"CUSTOM-FIXTURE", "/unknown-fixture", http.StatusNotFound},
		{http.MethodOptions, "/metric-test/7630", http.StatusNoContent},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		require.Equal(t, tc.code, rec.Code)
	}

	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	require.Contains(t, body, `goget_http_requests_total{code="200",method="GET",route="/metric-test/{id:[0-9]+}"} 1`)
	require.Contains(t, body, `goget_http_requests_total{code="503",method="GET",route="/metric-test/{id:[0-9]+}"} 1`)
	require.Contains(t, body, `goget_http_requests_total{code="404",method="OTHER",route="unmatched"}`)
	require.Contains(t, body, `goget_http_requests_total{code="204",method="OPTIONS",route="cors_preflight"}`)
	require.Contains(t, body, `goget_http_request_duration_seconds_count{method="GET",route="/metric-test/{id:[0-9]+}"} 2`)
	require.NotContains(t, body, "/metric-test/7628")
	require.NotContains(t, body, "secret-fixture")
	require.NotContains(t, body, "CUSTOM-FIXTURE")
}
