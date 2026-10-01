package config

import (
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("FRONTEND_ORIGIN", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("COOKIE_SECURE", "")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.FrontendOrigin != "http://localhost:5173" {
		t.Errorf("FrontendOrigin = %q, want http://localhost:5173", cfg.FrontendOrigin)
	}
	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/gogetdb?sslmode=disable" {
		t.Errorf("DatabaseURL = %q, want default", cfg.DatabaseURL)
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure = true, want false")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("FRONTEND_ORIGIN", "https://example.com")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/prod")
	t.Setenv("COOKIE_SECURE", "true")

	cfg := Load()

	if cfg.Port != "9999" {
		t.Errorf("Port = %q, want 9999", cfg.Port)
	}
	if cfg.FrontendOrigin != "https://example.com" {
		t.Errorf("FrontendOrigin = %q", cfg.FrontendOrigin)
	}
	if cfg.DatabaseURL != "postgres://user:pass@db:5432/prod" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if !cfg.CookieSecure {
		t.Error("CookieSecure = false, want true")
	}
}

func TestLoad_CookieSecure_False(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "false")

	cfg := Load()

	if cfg.CookieSecure {
		t.Error("CookieSecure = true, want false")
	}
}

func TestLoad_CookieSecure_InvalidValue(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "yes")

	cfg := Load()

	if cfg.CookieSecure {
		t.Error("CookieSecure = true при значении 'yes', want false")
	}
}

func TestGetEnv_Fallback(t *testing.T) {
	t.Setenv("SOME_UNKNOWN_VAR", "")

	if got := getEnv("SOME_UNKNOWN_VAR", "default"); got != "default" {
		t.Errorf("getEnv = %q, want default", got)
	}
}

func TestGetEnv_Present(t *testing.T) {
	t.Setenv("SOME_VAR", "custom")

	if got := getEnv("SOME_VAR", "default"); got != "custom" {
		t.Errorf("getEnv = %q, want custom", got)
	}
}
