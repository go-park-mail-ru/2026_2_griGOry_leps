package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gateway"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/platform/httpserver"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(logger.New(os.Stdout, logger.ParseLevel(os.Getenv("LOG_LEVEL"))))
	handler, err := gateway.New(
		httpserver.Env("AUTH_URL", "http://localhost:8081"),
		httpserver.Env("ADS_URL", "http://localhost:8082"),
		httpserver.Env("SEARCH_URL", "http://localhost:8083"),
		httpserver.Env("FRONTEND_ORIGIN", "http://localhost:5173"),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := httpserver.Run("gateway", httpserver.Env("GATEWAY_PORT", "8080"), handler); err != nil {
		log.Fatal(err)
	}
}
