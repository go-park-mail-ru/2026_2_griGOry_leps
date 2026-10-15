package config

import "os"

type Config struct {
	Port           string
	FrontendOrigin string
	CookieSecure   bool
	DatabaseURL    string
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		FrontendOrigin: getEnv("FRONTEND_ORIGIN", "http://localhost:5173"),
		CookieSecure:   getEnv("COOKIE_SECURE", "false") == "true",
		DatabaseURL:    getEnv("GOOSE_DBSTRING", "host=127.0.0.1 port=5432 user=postgres password=12345 dbname=goget sslmode=disable"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
