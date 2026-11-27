package metrics

import (
	"context"

	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	grpcRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goget_grpc_server_requests_total",
		Help: "Number of completed unary gRPC calls",
	}, []string{"rpc_method", "code"})

	grpcRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "goget_grpc_server_request_duration_seconds",
		Help: "Duration of unary gRPC calls",
	}, []string{"rpc_method"})
)

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
