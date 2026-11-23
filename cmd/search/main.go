package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/config"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/platform/httpserver"
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
	router := delivery.NewSearchRouter(cfg.FrontendOrigin)
	return httpserver.Run("search", "127.0.0.1", httpserver.Env("SEARCH_PORT", "8083"), router)
}
