package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
)

func TestWriteJSON_EncodeErrorLoggedWithRequestID(t *testing.T) {
	buf := captureLogs(t)
	ctx := logger.WithRequestID(context.Background(), "resp-1")

	writeJSON(ctx, httptest.NewRecorder(), http.StatusOK, make(chan int))

	records := logRecords(t, buf)
	require.Len(t, records, 1)
	require.Equal(t, "write json response", records[0]["msg"])
	require.Equal(t, "resp-1", records[0]["request_id"])
}
