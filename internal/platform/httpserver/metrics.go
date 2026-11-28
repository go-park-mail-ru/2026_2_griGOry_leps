package httpserver

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func metricsEnabled(name string) bool {
	switch name {
	case "auth", "ads", "search", "onlineconf":
		return true
	default:
		return false
	}
}

func withMetrics(next http.Handler) http.Handler {
	metrics := promhttp.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" && r.Method == http.MethodGet {
			metrics.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
