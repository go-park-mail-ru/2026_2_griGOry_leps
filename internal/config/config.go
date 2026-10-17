package config

import (
	"log/slog"
	"os"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
)

type Config struct {
	Port           string
	FrontendOrigin string
	CookieSecure   bool
	LogLevel       slog.Level
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		FrontendOrigin: getEnv("FRONTEND_ORIGIN", "http://localhost:5173"),
		CookieSecure:   getEnv("COOKIE_SECURE", "false") == "true",
		LogLevel:       logger.ParseLevel(getEnv("LOG_LEVEL", "info")),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
