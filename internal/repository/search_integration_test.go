package repository

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	adsv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/ads/v1"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
)

func TestPublishedSearch(t *testing.T) {
	db := newTestPool(t)

	_, err := db.Exec(`
		INSERT INTO user_account (first_name, email, phone_number, password_hash)
		VALUES ('Test', 'test@example.org', '+79990000001', 'unused');

		INSERT INTO category (category_id, name, slug) OVERRIDING SYSTEM VALUE
		VALUES (1, 'Техника', 'electronics'), (2, 'Книги', 'books');

		INSERT INTO ad (user_id, title, description, status, price, published_at, category_id, city, has_delivery)
		VALUES
			(1, 'Камера A', '',           'active',     100, '2026-01-01', 1, 'Москва', true),
			(1, 'Комплект', 'КАМЕРА 4K', 'active',     200, '2026-01-01', 1, 'Пермь',  false),
			(1, 'Камера C', '',           'draft',      300, NULL,         1, 'Москва', false),
			(1, 'Книга',    '',           'active',     400, '2026-02-01', 2, 'Москва', true),
			(1, 'Камера D', '',           'closed',     500, '2026-03-01', 1, 'Москва', false),
			(1, 'Камера E', '',           'moderation', 100, NULL,         1, 'Москва', true),
			(1, 'Камера F', '',           'rejected',   100, NULL,         1, 'Москва', true);
	`)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	ads := usecase.NewAdUsecase(NewAdRepository(db))
	adsv1.RegisterAdsServiceServer(server, delivery.NewAdsGRPCServer(ads))

	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(
		listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	router := delivery.NewSearchRouter("", adsv1.NewAdsServiceClient(conn))

	tests := []struct {
		name   string
		query  string
		titles []string
		total  int
	}{
		{"первая страница", "?q=+кАмЕрА+&limit=1", []string{"Комплект"}, 2},
		{"вторая страница", "?q=камера&limit=1&offset=1", []string{"Камера A"}, 2},
		{"за последней страницей", "?q=камера&offset=2", []string{}, 2},
		{"пустой запрос", "", []string{"Книга", "Комплект", "Камера A"}, 3},
		{"одни пробелы", "?q=+++", []string{"Книга", "Комплект", "Камера A"}, 3},
		{"нет совпадений", "?q=телефон", []string{}, 0},
		{"город, цена и доставка", "?q=камера&city=москва&price_min=1&price_max=1.50&has_delivery=true", []string{"Камера A"}, 1},
		{"явно без доставки", "?q=камера&has_delivery=false", []string{"Комплект"}, 1},
		{"категория и сортировка", "?category_id=1&sort=price_asc", []string{"Камера A", "Комплект"}, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/search"+tc.query, nil)
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			var page struct {
				Items []struct{ Title string }
				Total int
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
			require.NotNil(t, page.Items)
			require.Equal(t, tc.total, page.Total)

			titles := make([]string, 0, len(page.Items))
			for _, item := range page.Items {
				titles = append(titles, item.Title)
			}
			require.Equal(t, tc.titles, titles)
		})
	}

	firstPage := httptest.NewRecorder()
	router.ServeHTTP(firstPage, httptest.NewRequest(http.MethodGet, "/api/search?q=камера&limit=1", nil))
	require.Equal(t, http.StatusOK, firstPage.Code)
	var first struct {
		NextCursor string `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(firstPage.Body.Bytes(), &first))
	require.NotEmpty(t, first.NextCursor)
	secondPage := httptest.NewRecorder()
	path := "/api/search?q=камера&limit=1&cursor=" + url.QueryEscape(first.NextCursor)
	router.ServeHTTP(secondPage, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, secondPage.Code, secondPage.Body.String())
	require.Contains(t, secondPage.Body.String(), "Камера A")
	require.NotContains(t, secondPage.Body.String(), "Комплект")

	for _, query := range []string{"q=%ZZ", "q=%FF", "city=%00", "limit=-1", "has_delivery=maybe", "price_min=3&price_max=1", "sort=unknown", "cursor=invalid"} {
		t.Run("invalid/"+query, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/search?"+query, nil))
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}
