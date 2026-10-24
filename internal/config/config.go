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
	CSRFSecret     string
	DatabaseURL    string
	UploadDir      string
	SMTPHost       string
	SMTPPort       string
	SMTPUser       string
	SMTPPassword   string
	MailFrom       string
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		FrontendOrigin: getEnv("FRONTEND_ORIGIN", "http://localhost:5173"),
		CookieSecure:   getEnv("COOKIE_SECURE", "false") == "true",
		LogLevel:       logger.ParseLevel(getEnv("LOG_LEVEL", "info")),
		CSRFSecret:     os.Getenv("CSRF_SECRET"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:12345@127.0.0.1:5432/goget?sslmode=disable"),
		UploadDir:      getEnv("UPLOAD_DIR", "uploads"),
		SMTPHost:       os.Getenv("SMTP_HOST"),
		SMTPPort:       getEnv("SMTP_PORT", "587"),
		SMTPUser:       os.Getenv("SMTP_USER"),
		SMTPPassword:   os.Getenv("SMTP_PASSWORD"),
		MailFrom:       getEnv("MAIL_FROM", "Go&Get <noreply@goget.pw>"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
