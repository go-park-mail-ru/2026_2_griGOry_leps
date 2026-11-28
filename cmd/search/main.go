package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	authclient "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/client/auth"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/config"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/onlineconf"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/platform/httpserver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	adsv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/ads/v1"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	slog.SetDefault(logger.New(os.Stdout, cfg.LogLevel))
	if err := run(cfg); err != nil {
		slog.Error("search service failed", "err", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	addr := httpserver.Env("ADS_GRPC_ADDR", "127.0.0.1:9092")

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	ads := adsv1.NewAdsServiceClient(conn)
	auth, err := authclient.New(httpserver.Env("AUTH_GRPC_ADDR", "127.0.0.1:9091"), 2*time.Second)
	if err != nil {
		return err
	}
	defer auth.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	settings := onlineconf.NewSearchCache(os.Getenv("ONLINECONF_SEARCH_SNAPSHOT_URL"))
	go settings.Run(ctx, 15*time.Second)
	router := delivery.NewSearchRouter(cfg.FrontendOrigin, ads, delivery.NewSessionMiddleware(auth), settings)

	return httpserver.RunContext(ctx, "search", "127.0.0.1", httpserver.Env("SEARCH_PORT", "8083"), router)
}
