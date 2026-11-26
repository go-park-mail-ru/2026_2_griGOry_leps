package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gateway"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/platform/httpserver"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(logger.New(os.Stdout, logger.ParseLevel(os.Getenv("LOG_LEVEL"))))
	handler, err := gateway.New(
		httpserver.Env("AUTH_URL", "http://127.0.0.1:8081"),
		httpserver.Env("ADS_URL", "http://127.0.0.1:8082"),
		httpserver.Env("SEARCH_URL", "http://127.0.0.1:8083"),
		httpserver.Env("FRONTEND_ORIGIN", "http://localhost:5173"),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := run(delivery.WithGatewayMetrics(handler)); err != nil {
		log.Fatal(err)
	}
}

func run(handler http.Handler) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metricsMux := http.NewServeMux()
	metricsMux.Handle("GET /metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	metricsAddr := net.JoinHostPort("127.0.0.1", httpserver.Env("GATEWAY_METRICS_PORT", "9080"))
	listener, err := net.Listen("tcp", metricsAddr)
	if err != nil {
		return err
	}

	metricsDone := make(chan error, 1)
	go func() {
		slog.Info("gateway metrics listening", "address", listener.Addr().String())
		err := metricsServer.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		metricsDone <- err
		stop()
	}()

	apiErr := httpserver.RunContext(
		ctx,
		"gateway",
		os.Getenv("GATEWAY_HOST"),
		httpserver.Env("GATEWAY_PORT", "8080"),
		handler,
	)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdownErr := metricsServer.Shutdown(shutdownCtx)
	return errors.Join(apiErr, shutdownErr, <-metricsDone)
}
