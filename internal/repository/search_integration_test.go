package repository

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
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

		INSERT INTO ad (user_id, title, description, status, price, published_at)
		VALUES
			(1, 'Камера A', '',           'active', 100, '2026-01-01'),
			(1, 'Комплект', 'КАМЕРА 4K', 'active', 200, '2026-01-01'),
			(1, 'Камера C', '',           'draft',  300, NULL),
			(1, 'Книга',    '',           'active', 400, '2026-02-01'),
			(1, 'Камера D', '',           'closed', 500, '2026-03-01');
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
}
