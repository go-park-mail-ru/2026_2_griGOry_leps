package delivery

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
)

var generatedID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(logger.New(&buf, slog.LevelDebug))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func logRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		records = append(records, record)
	}
	return records
}

func accessRecord(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	for _, record := range logRecords(t, buf) {
		if record["msg"] == "http request" {
			return record
		}
	}
	t.Fatalf("no access log record in %q", buf.String())
	return nil
}

func TestRequestID_Generated(t *testing.T) {
	var seen string
	h := requestIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = logger.RequestID(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	id := rec.Header().Get(requestIDHeader)
	require.Regexp(t, generatedID, id)
	require.Equal(t, id, seen)
}

func TestRequestID_KeepsValidIncoming(t *testing.T) {
	var seen string
	h := requestIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = logger.RequestID(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(requestIDHeader, "abc-123_X")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, "abc-123_X", rec.Header().Get(requestIDHeader))
	require.Equal(t, "abc-123_X", seen)
}

func TestRequestID_RejectsInvalidIncoming(t *testing.T) {
	h := requestIDMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	for _, incoming := range []string{"bad id", "evil\"}", strings.Repeat("a", 65)} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(requestIDHeader, incoming)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Regexp(t, generatedID, rec.Header().Get(requestIDHeader), incoming)
	}
}

func TestAccessLog_RecordsRequest(t *testing.T) {
	buf := captureLogs(t)
	h := requestIDMiddleware(accessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	})))

	req := httptest.NewRequest(http.MethodPost, "/api/register?x=1", nil)
	req.Header.Set(requestIDHeader, "req-1")
	req.Header.Set("User-Agent", "test-agent")
	h.ServeHTTP(httptest.NewRecorder(), req)

	record := accessRecord(t, buf)
	require.Equal(t, "INFO", record["level"])
	require.Equal(t, "req-1", record["request_id"])
	require.Equal(t, http.MethodPost, record["method"])
	require.Equal(t, "/api/register", record["path"])
	require.EqualValues(t, http.StatusCreated, record["status"])
	require.EqualValues(t, 5, record["bytes"])
	require.Equal(t, "test-agent", record["user_agent"])
	require.Contains(t, record, "duration_ms")
	require.Contains(t, record, "remote_addr")
}

func TestAccessLog_DefaultStatus200(t *testing.T) {
	buf := captureLogs(t)
	h := accessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	require.EqualValues(t, http.StatusOK, accessRecord(t, buf)["status"])
}

func TestAccessLog_ServerErrorLevel(t *testing.T) {
	buf := captureLogs(t)
	h := accessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, "ERROR", accessRecord(t, buf)["level"])
}
