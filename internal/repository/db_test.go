//go:build integration

package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Каждый тест получает отдельную схему и реальные таблицы из миграции.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "set TEST_DATABASE_URL to a dedicated test database")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(admin.Close)

	var token [8]byte
	_, err = rand.Read(token[:])
	require.NoError(t, err)
	schema := pgx.Identifier{"repo_test_" + hex.EncodeToString(token[:])}.Sanitize()
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, err)
	})

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// Up всех миграций по порядку номеров
	files, err := filepath.Glob("../../migrations/*.sql")
	require.NoError(t, err)
	sort.Slice(files, func(i, j int) bool { return migrationNumber(t, files[i]) < migrationNumber(t, files[j]) })
	for _, file := range files {
		migration, err := os.ReadFile(file)
		require.NoError(t, err)
		up, _, ok := strings.Cut(string(migration), "-- +goose Down")
		require.True(t, ok, file)
		_, err = pool.Exec(ctx, up)
		require.NoError(t, err, file)
	}
	return pool
}

func migrationNumber(t *testing.T, file string) int {
	t.Helper()
	prefix, _, _ := strings.Cut(filepath.Base(file), "_")
	n, err := strconv.Atoi(prefix)
	require.NoError(t, err, file)
	return n
}

func newTestSessionRepository(t *testing.T) (*SessionRepository, int32) {
	t.Helper()
	pool := newTestPool(t)
	user, err := NewUserRepository(pool).Create(context.Background(), "session@example.com", "hash", "Иван", "+79001234567")
	require.NoError(t, err)
	return NewSessionRepository(pool), user.ID
}

func newTestAdRepository(t *testing.T, ads []domain.Ad) *AdRepository {
	t.Helper()
	pool := newTestPool(t)
	ctx := context.Background()
	user, err := NewUserRepository(pool).Create(ctx, "seller@example.com", "hash", "Иван", "+79001234567")
	require.NoError(t, err)
	// Фиксированные ID принадлежат только изолированным тестовым данным.
	_, err = pool.Exec(ctx, `INSERT INTO category (category_id, name, slug)
		OVERRIDING SYSTEM VALUE VALUES (1, 'Транспорт', 'transport'), (2, 'Книги', 'books')`)
	require.NoError(t, err)
	for _, ad := range ads {
		_, err = pool.Exec(ctx, `INSERT INTO ad
			(id, user_id, category_id, title, description, price, city, has_delivery, status, created_at)
			OVERRIDING SYSTEM VALUE VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			ad.ID, user.ID, ad.CategoryID, ad.Title, ad.Description, ad.Price,
			ad.City, ad.HasDelivery, ad.Status, ad.CreatedAt)
		require.NoError(t, err)
	}
	return NewAdRepository(pool)
}
