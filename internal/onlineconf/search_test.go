package onlineconf

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSearchCacheRetainsSnapshotUntilRecovery(t *testing.T) {
	var reply atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("scope") != "search" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch reply.Load() {
		case 0:
			w.WriteHeader(http.StatusServiceUnavailable)
		case 1:
			_, _ = io.WriteString(w, `[{"id":1,"scope":"search","kind":"config","name":"max_query_length","value":20},{"id":2,"scope":"search","kind":"flag","name":"price_sort","value":false}]`)
		case 2:
			<-r.Context().Done()
		case 3:
			_, _ = io.WriteString(w, `[{"id":3,"scope":"ads","kind":"config","name":"max_query_length","value":5},{"id":2,"scope":"search","kind":"flag","name":"price_sort","value":true}]`)
		}
	}))
	t.Cleanup(server.Close)

	cache := NewSearchCache(server.URL + "/v1/entries")
	require.ErrorContains(t, cache.refresh(t.Context()), "503")
	require.False(t, cache.Snapshot().PriceSortEnabled(7))
	require.Zero(t, cache.Snapshot().MaxQueryLength)

	reply.Store(1)
	require.NoError(t, cache.refresh(t.Context()))
	require.EqualValues(t, 20, cache.Snapshot().MaxQueryLength)
	require.False(t, cache.Snapshot().PriceSortEnabled(7))

	reply.Store(0)
	require.ErrorContains(t, cache.refresh(t.Context()), "503")
	require.EqualValues(t, 20, cache.Snapshot().MaxQueryLength)

	reply.Store(2)
	cache.client.Timeout = 100 * time.Millisecond
	require.ErrorContains(t, cache.refresh(t.Context()), "OnlineConf request failed")
	require.EqualValues(t, 20, cache.Snapshot().MaxQueryLength)
	require.False(t, cache.Snapshot().PriceSortEnabled(7))

	reply.Store(3)
	cache.client.Timeout = time.Second
	require.NoError(t, cache.refresh(t.Context()))
	require.Zero(t, cache.Snapshot().MaxQueryLength)
	require.True(t, cache.Snapshot().PriceSortEnabled(7))
}
