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
	gateway, err := New(upstream.URL, upstream.URL, upstream.URL, upstream.URL, "")
	require.NoError(t, err)

	for _, path := range []string{"/metrics", "/metrics/", "/%6detrics"} {
		response := httptest.NewRecorder()
		gateway.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusNotFound, response.Code, path)
	}
}

func TestOnlineConfRouted(t *testing.T) {
	upstream := func(name string) *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Upstream", name)
			w.Header().Set("X-Cookie", r.Header.Get("Cookie"))
			w.Header().Set("X-Path", r.URL.Path)
		}))
		t.Cleanup(server.Close)
		return server
	}
	auth, ads, search, onlineconf := upstream("auth"), upstream("ads"), upstream("search"), upstream("onlineconf")
	handler, err := New(auth.URL, ads.URL, search.URL, onlineconf.URL, "")
	require.NoError(t, err)
	for _, tc := range []struct{ path, want string }{
		{"/api/onlineconf/admin/me", "onlineconf"},
		{"/api/onlineconf", "onlineconf"},
		{"/api/onlineconf/", "onlineconf"},
		{"/api/onlineconfx", "ads"},
		{"/api/search", "search"},
		{"/api/search/settings", "search"},
		{"/api/login", "auth"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "s"})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, tc.want, rec.Header().Get("X-Upstream"), tc.path)
		require.Equal(t, "session_id=s", rec.Header().Get("X-Cookie"))
		require.Equal(t, tc.path, rec.Header().Get("X-Path"))
	}
}

func TestMetricRouteOnlineConf(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"/api/onlineconf/admin/me", "onlineconf"},
		{"/api/onlineconf", "onlineconf"},
		{"/api/onlineconfx", "ads"},
		{"/api/ads/../onlineconf/admin/me", "onlineconf"},
		{"/api/search/settings", "search"},
	} {
		require.Equal(t, tc.want, MetricRoute(httptest.NewRequest(http.MethodGet, tc.path, nil)))
	}
}

func TestIsOnlineConfPath(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/api/onlineconf", true},
		{"/api/onlineconf/admin/me", true},
		{"/api/onlineconf/", true},
		{"/api/onlineconfx", false},
		{"/api/search", false},
	} {
		require.Equal(t, tc.want, IsOnlineConfPath(tc.path), tc.path)
	}
}

func TestNew_InvalidOnlineConfURL(t *testing.T) {
	_, err := New("http://auth.test", "http://ads.test", "http://search.test", ":invalid", "")
	require.Error(t, err)
}
