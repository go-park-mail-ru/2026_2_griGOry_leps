package delivery

import (
	"net/http"
	"strconv"

	"github.com/felixge/httpsnoop"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gateway"
)

// WithGatewayMetrics records locally handled and proxied requests in the same
// collectors used by the service routers.
func WithGatewayMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := gateway.MetricRoute(r)
		method := r.Method
		switch method {
		case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
			http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		default:
			method = "OTHER"
		}
		result := httpsnoop.CaptureMetrics(next, w, r)
		requestCount.WithLabelValues(method, route, strconv.Itoa(result.Code)).Inc()
		requestDuration.WithLabelValues(method, route).Observe(result.Duration.Seconds())
	})
}
