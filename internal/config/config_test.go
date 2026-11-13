package config

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_ModeratorIDs(t *testing.T) {
	for _, value := range []string{"", " ", "1,wrong", "0", "-1", "2147483648", "1,"} {
		t.Setenv("MODERATOR_USER_IDS", value)
		require.Empty(t, Load().ModeratorUserIDs, value)
	}
	t.Setenv("MODERATOR_USER_IDS", "1, 42")
	require.Equal(t, []int32{1, 42}, Load().ModeratorUserIDs)
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("FRONTEND_ORIGIN", "")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("UPLOAD_DIR", "")

	cfg := Load()

	require.Equal(t, "8080", cfg.Port)
	require.Equal(t, "http://localhost:5173", cfg.FrontendOrigin)
	require.False(t, cfg.CookieSecure)
	require.Equal(t, "uploads", cfg.UploadDir)
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("FRONTEND_ORIGIN", "https://example.com")
	t.Setenv("COOKIE_SECURE", "true")

	cfg := Load()

	require.Equal(t, "9999", cfg.Port)
	require.Equal(t, "https://example.com", cfg.FrontendOrigin)
	require.True(t, cfg.CookieSecure)
}

func TestLoad_LogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "")
	require.Equal(t, slog.LevelInfo, Load().LogLevel)

	t.Setenv("LOG_LEVEL", "debug")
	require.Equal(t, slog.LevelDebug, Load().LogLevel)
}

func TestLoad_CSRFSecret(t *testing.T) {
	t.Setenv("CSRF_SECRET", "")
	require.Empty(t, Load().CSRFSecret)

	t.Setenv("CSRF_SECRET", "top-secret")
	require.Equal(t, "top-secret", Load().CSRFSecret)
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

func TestLoad_DatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	require.Contains(t, Load().DatabaseURL, "127.0.0.1:5432/goget")

	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x")
	require.Equal(t, "postgres://u:p@db:5432/x", Load().DatabaseURL)
}

func TestLoad_DeliveryPrices(t *testing.T) {
	t.Setenv("DELIVERY_COURIER_PRICE", "")
	t.Setenv("DELIVERY_PICKUP_POINT_PRICE", "")
	cfg := Load()
	require.Equal(t, int64(39000), cfg.CourierPrice)
	require.Equal(t, int64(19000), cfg.PickupPointPrice)

	t.Setenv("DELIVERY_COURIER_PRICE", "450.5")
	t.Setenv("DELIVERY_PICKUP_POINT_PRICE", "-1")
	cfg = Load()
	require.Equal(t, int64(45050), cfg.CourierPrice)
	require.Equal(t, int64(19000), cfg.PickupPointPrice)
}
