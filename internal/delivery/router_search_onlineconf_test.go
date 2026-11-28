package delivery

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gateway"
	adsv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/ads/v1"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/onlineconf"
)

type onlineConfSearchAds struct {
	lastRequest atomic.Pointer[adsv1.SearchRequest]
}

func (a *onlineConfSearchAds) SearchPublished(_ context.Context, req *adsv1.SearchRequest, _ ...grpc.CallOption) (*adsv1.SearchPage, error) {
	a.lastRequest.Store(req)
	return &adsv1.SearchPage{}, nil
}

func TestSearchOnlineConfRefreshesSettingsAndSearchTogether(t *testing.T) {
	snapshot := func(limit int, flag, color string) string {
		return fmt.Sprintf(`[{"scope":"search","name":"max_query_length","kind":"config","value":%d},{"scope":"search","name":"price_sort","kind":"flag","value":%s},{"scope":"search","name":"search_button_color","kind":"config","value":%s}]`, limit, flag, color)
	}
	var body atomic.Value
	body.Store(snapshot(3, `{"rollout_percent":0,"included_user_ids":[21]}`, `"orange"`))
	var polls atomic.Int32
	config := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polls.Add(1)
		fmt.Fprint(w, body.Load().(string))
	}))
	t.Cleanup(config.Close)

	ads := &onlineConfSearchAds{}
	auth := NewMockAuthUsecase(gomock.NewController(t))
	auth.EXPECT().Me(gomock.Any(), "buyer").Return(domain.User{ID: 21}, nil).AnyTimes()
	auth.EXPECT().Me(gomock.Any(), "seller").Return(domain.User{ID: 22}, nil).AnyTimes()
	auth.EXPECT().Me(gomock.Any(), "rollout-user-4").Return(domain.User{ID: 4}, nil).AnyTimes()
	auth.EXPECT().Me(gomock.Any(), "rollout-user-12").Return(domain.User{ID: 12}, nil).AnyTimes()
	auth.EXPECT().Me(gomock.Any(), "expired").Return(domain.User{}, domain.ErrSessionExpired).AnyTimes()
	settings := onlineconf.NewSearchCache(config.URL)
	search := httptest.NewServer(NewSearchRouter("http://front.test", ads, NewSessionMiddleware(auth), settings))
	t.Cleanup(search.Close)
	router, err := gateway.New(config.URL, config.URL, search.URL, config.URL, "http://front.test")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		settings.Run(ctx, 15*time.Millisecond)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	require.Eventually(t, func() bool { return settings.Snapshot().MaxQueryLength == 3 }, 3*time.Second, 5*time.Millisecond)

	get := func(path, session string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if session != "" {
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session})
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	rec := get("/api/search/settings", "buyer")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.JSONEq(t, `{"max_query_length":3,"price_sort_enabled":true,"search_button_color":"orange"}`, rec.Body.String())
	for _, session := range []string{"seller", "", "expired"} {
		rec = get("/api/search/settings", session)
		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"max_query_length":3,"price_sort_enabled":false,"search_button_color":"orange"}`, rec.Body.String())
		rec = get("/api/search?sort=price_asc&user_id=21", session)
		require.Equal(t, http.StatusForbidden, rec.Code)
		require.JSONEq(t, `{"error":"price sorting disabled"}`, rec.Body.String())
	}
	require.Nil(t, ads.lastRequest.Load())

	rec = get("/api/search?sort=price_asc&q="+url.QueryEscape(" Я🙂я "), "buyer")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Equal(t, "Я🙂я", ads.lastRequest.Load().Query)
	rec = get("/api/search?q="+url.QueryEscape("Я🙂яя"), "buyer")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"search query too long","field":"q"}`, rec.Body.String())
	require.Equal(t, http.StatusOK, get("/api/search?sort=newest&price_min=10", "no-auth-call").Code)

	body.Store(snapshot(2, `{"rollout_percent":100,"excluded_user_ids":[21]}`, `"blue"`))
	require.Eventually(t, func() bool { return settings.Snapshot().MaxQueryLength == 2 }, 3*time.Second, 5*time.Millisecond)
	require.Equal(t, http.StatusForbidden, get("/api/search?sort=price_desc", "buyer").Code)
	require.Equal(t, http.StatusOK, get("/api/search?sort=price_desc", "seller").Code)
	require.Equal(t, http.StatusOK, get("/api/search?sort=price_desc", "").Code)

	body.Store(snapshot(1, `{"rollout_percent":101}`, `"orange"`))
	wantPolls := polls.Load() + 2
	require.Eventually(t, func() bool { return polls.Load() >= wantPolls }, 3*time.Second, 5*time.Millisecond)
	rec = get("/api/search/settings", "buyer")
	require.JSONEq(t, `{"max_query_length":2,"price_sort_enabled":false,"search_button_color":"blue"}`, rec.Body.String())

	body.Store(snapshot(4, `{"rollout_percent":25}`, `{"color":"orange"}`))
	require.Eventually(t, func() bool { return settings.Snapshot().MaxQueryLength == 4 }, 3*time.Second, 5*time.Millisecond)
	rec = get("/api/search/settings", "rollout-user-4")
	require.JSONEq(t, `{"max_query_length":4,"price_sort_enabled":false,"search_button_color":"blue"}`, rec.Body.String())
	require.Equal(t, http.StatusForbidden, get("/api/search?sort=price_desc", "rollout-user-4").Code)
	rec = get("/api/search/settings", "rollout-user-12")
	require.JSONEq(t, `{"max_query_length":4,"price_sort_enabled":true,"search_button_color":"blue"}`, rec.Body.String())
	require.Equal(t, http.StatusOK, get("/api/search?sort=price_desc", "rollout-user-12").Code)
}

func TestSearchOnlineConfStartupDefaults(t *testing.T) {
	for _, tc := range []struct {
		name, url, settings string
		priceSortStatus     int
	}{
		{"no URL configured", "", `{"max_query_length":null,"price_sort_enabled":true,"search_button_color":"blue"}`, http.StatusOK},
		{"before first snapshot", "http://127.0.0.1:1/v1/entries", `{"max_query_length":null,"price_sort_enabled":false,"search_button_color":"blue"}`, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := NewSearchRouter("", &onlineConfSearchAds{}, nil, onlineconf.NewSearchCache(tc.url))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/search/settings", nil))
			require.Equal(t, http.StatusOK, rec.Code)
			require.JSONEq(t, tc.settings, rec.Body.String())
			rec = httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/search?sort=price_asc", nil))
			require.Equal(t, tc.priceSortStatus, rec.Code)
		})
	}
}
