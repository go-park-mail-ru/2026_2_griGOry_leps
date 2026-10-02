package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("FRONTEND_ORIGIN", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("COOKIE_SECURE", "")

	cfg := Load()

	require.Equal(t, "8080", cfg.Port)
	require.Equal(t, "http://localhost:5173", cfg.FrontendOrigin)
	require.Equal(t, "postgres://postgres:postgres@localhost:5432/gogetdb?sslmode=disable", cfg.DatabaseURL)
	require.False(t, cfg.CookieSecure)
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("FRONTEND_ORIGIN", "https://example.com")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/prod")
	t.Setenv("COOKIE_SECURE", "true")

	cfg := Load()

	require.Equal(t, "9999", cfg.Port)
	require.Equal(t, "https://example.com", cfg.FrontendOrigin)
	require.Equal(t, "postgres://user:pass@db:5432/prod", cfg.DatabaseURL)
	require.True(t, cfg.CookieSecure)
}

func TestLoad_CookieSecure_False(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "false")

	cfg := Load()

	require.False(t, cfg.CookieSecure)
}

func TestLoad_CookieSecure_InvalidValue(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "yes")

	cfg := Load()

	require.False(t, cfg.CookieSecure)
}

func TestGetEnv_Fallback(t *testing.T) {
	t.Setenv("SOME_UNKNOWN_VAR", "")

	require.Equal(t, "default", getEnv("SOME_UNKNOWN_VAR", "default"))
}

func TestGetEnv_Present(t *testing.T) {
	t.Setenv("SOME_VAR", "custom")

	require.Equal(t, "custom", getEnv("SOME_VAR", "default"))
}
