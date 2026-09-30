package config

import "os"

type Config struct {
	Port           string
	FrontendOrigin string
	CookieSecure   bool
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		FrontendOrigin: getEnv("FRONTEND_ORIGIN", "http://localhost:5173"),
		CookieSecure:   getEnv("COOKIE_SECURE", "false") == "true",
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
