package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetricsNotForwarded(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	gateway, err := New(upstream.URL, upstream.URL, upstream.URL, "")
	require.NoError(t, err)

	for _, path := range []string{"/metrics", "/metrics/", "/%6detrics"} {
		response := httptest.NewRecorder()
		gateway.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusNotFound, response.Code, path)
	}
}
