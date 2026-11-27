package delivery

import (
	"context"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

var requestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "goget_http_requests_total",
	Help: "HTTP requests by method, route template, and response code.",
}, []string{"method", "route", "code"})

var requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "goget_http_request_duration_seconds",
	Help:    "HTTP response duration in seconds by method and route template.",
	Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 10, 30},
}, []string{"method", "route"})

func init() {
	prometheus.MustRegister(requestCount, requestDuration)
}

type metricRouteKey struct{}

func captureMetricRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if destination, ok := r.Context().Value(metricRouteKey{}).(*atomic.Pointer[mux.Route]); ok {
			destination.Store(mux.CurrentRoute(r))
		}
		next.ServeHTTP(w, r)
	})
}

func trackMetricRoute(r *http.Request) (*http.Request, *atomic.Pointer[mux.Route]) {
	// TimeoutHandler can return while the handler goroutine is still running.
	route := new(atomic.Pointer[mux.Route])
	return r.WithContext(context.WithValue(r.Context(), metricRouteKey{}, route)), route
}

func observeHTTPRequest(method string, route *mux.Route, code int, duration time.Duration) {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace:
	default:
		method = "OTHER"
	}
	template := "unmatched"
	if route != nil {
		if path, err := route.GetPathTemplate(); err == nil {
			template = path
		}
	} else if method == http.MethodOptions && code == http.StatusNoContent {
		template = "cors_preflight"
	}
	requestCount.WithLabelValues(method, template, strconv.Itoa(code)).Inc()
	requestDuration.WithLabelValues(method, template).Observe(duration.Seconds())
}
