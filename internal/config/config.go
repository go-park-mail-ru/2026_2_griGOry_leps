package config

import (
	"log/slog"
	"os"
	"regexp"
	"strconv"

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

	CourierPrice     int64
	PickupPointPrice int64
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

		CourierPrice:     getPrice("DELIVERY_COURIER_PRICE", 39000),
		PickupPointPrice: getPrice("DELIVERY_PICKUP_POINT_PRICE", 19000),
	}
}

var priceFormat = regexp.MustCompile(`^(\d{1,9})(?:\.(\d{1,2}))?$`)

func getPrice(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	m := priceFormat.FindStringSubmatch(value)
	if m == nil {
		slog.Warn("invalid price in env, using default", "key", key, "value", value)
		return fallback
	}
	rubles, _ := strconv.ParseInt(m[1], 10, 64)
	kopecks, _ := strconv.ParseInt((m[2] + "00")[:2], 10, 64)
	return rubles*100 + kopecks
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
