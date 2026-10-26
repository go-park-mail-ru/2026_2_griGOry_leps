package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func decodeRecord(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
	return record
}

func TestNew_AddsRequestIDFromContext(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, slog.LevelInfo)

	log.InfoContext(WithRequestID(context.Background(), "abc"), "hello", "user_id", 7)

	record := decodeRecord(t, &buf)
	require.Equal(t, "hello", record["msg"])
	require.Equal(t, "abc", record["request_id"])
	require.EqualValues(t, 7, record["user_id"])
}

func TestNew_NoRequestIDWithoutContext(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, slog.LevelInfo)

	log.InfoContext(context.Background(), "hello")

	require.NotContains(t, decodeRecord(t, &buf), "request_id")
}

func TestNew_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, slog.LevelInfo)

	log.Debug("hidden")

	require.Zero(t, buf.Len())
}

func TestNew_WithAttrsKeepsRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, slog.LevelInfo).With("component", "auth")

	log.InfoContext(WithRequestID(context.Background(), "abc"), "hello", "method", "GET")

	record := decodeRecord(t, &buf)
	require.Equal(t, "auth", record["component"])
	require.Equal(t, "abc", record["request_id"])
	require.Equal(t, "GET", record["method"])
}

func TestRequestID_Empty(t *testing.T) {
	require.Empty(t, RequestID(context.Background()))
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"INFO":  slog.LevelInfo,
		"WARN":  slog.LevelWarn,
		"error": slog.LevelError,
		"":      slog.LevelInfo,
		"bogus": slog.LevelInfo,
	}

	for input, want := range cases {
		require.Equal(t, want, ParseLevel(input), input)
	}
}
