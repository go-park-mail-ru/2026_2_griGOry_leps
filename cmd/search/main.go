package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/config"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
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
	router := delivery.NewSearchRouter(cfg.FrontendOrigin, ads)

	return httpserver.Run("search", httpserver.Env("SEARCH_PORT", "8083"), router)
}
