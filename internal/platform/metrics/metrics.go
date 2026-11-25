package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goget_http_requests_total",
		Help: "Number of completed HTTP requests",
	}, []string{"method", "route", "code"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "goget_http_request_duration_seconds",
		Help: "Duration of HTTP requests",
	}, []string{"method", "route"})

	grpcRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goget_grpc_server_requests_total",
		Help: "Number of completed unary gRPC calls",
	}, []string{"rpc_method", "code"})

	grpcRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "goget_grpc_server_request_duration_seconds",
		Help: "Duration of unary gRPC calls",
	}, []string{"rpc_method"})
)

func RecordHTTPRequest(next http.Handler, routeFor func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := routeFor(r)
		metrics := httpsnoop.CaptureMetrics(next, w, r)
		httpRequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(metrics.Code)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(metrics.Duration.Seconds())
	})
}

func RecordGRPCRequest(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)

	code := status.Code(err)
	if err == context.Canceled || err == context.DeadlineExceeded {
		code = status.FromContextError(err).Code()
	}

	grpcRequestsTotal.WithLabelValues(info.FullMethod, code.String()).Inc()
	grpcRequestDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())

	return resp, err
}
