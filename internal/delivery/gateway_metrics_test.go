package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gateway"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

func TestBlockedRequestsGroupUnknownMethods(t *testing.T) {
	upstream := "http://upstream.invalid"
	gw, err := gateway.New(upstream, upstream, upstream, "")
	require.NoError(t, err)
	handler := WithGatewayMetrics(gw)
	for _, method := range []string{"OTHER", http.MethodGet} {
		requestCount.DeleteLabelValues(method, "blocked", "404")
		requestDuration.DeleteLabelValues(method, "blocked")
		t.Cleanup(func() {
			requestCount.DeleteLabelValues(method, "blocked", "404")
			requestDuration.DeleteLabelValues(method, "blocked")
		})
	}
	for _, method := range []string{"METHOD1", "METHOD2", http.MethodGet} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, "/internal", nil))
		require.Equal(t, http.StatusNotFound, rec.Code)
	}
	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	require.Contains(t, body, `goget_http_requests_total{code="404",method="OTHER",route="blocked"} 2`+"\n")
	require.Contains(t, body, `goget_http_request_duration_seconds_count{method="OTHER",route="blocked"} 2`+"\n")
	require.Contains(t, body, `goget_http_requests_total{code="404",method="GET",route="blocked"} 1`+"\n")
	require.Contains(t, body, `goget_http_request_duration_seconds_count{method="GET",route="blocked"} 1`+"\n")
	require.NotContains(t, body, `method="METHOD1"`)
	require.NotContains(t, body, `method="METHOD2"`)
}
