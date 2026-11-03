package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

// Каждый тест получает отдельную схему и реальные таблицы из миграции.
func newTestPool(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("integration test requires TEST_DATABASE_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	adminCfg, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	admin := stdlib.OpenDB(*adminCfg)
	err = admin.PingContext(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })

	var token [8]byte
	_, err = rand.Read(token[:])
	require.NoError(t, err)
	schema := pgx.Identifier{"repo_test_" + hex.EncodeToString(token[:])}.Sanitize()
	_, err = admin.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := admin.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, err)
	})

	cfg, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.RuntimeParams["search_path"] = schema
	cfg.RuntimeParams["statement_timeout"] = "5000"
	pool := stdlib.OpenDB(*cfg)
	err = pool.PingContext(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })

	files, err := filepath.Glob("../../migrations/*.sql")
	require.NoError(t, err)
	sort.Slice(files, func(i, j int) bool { return migrationNumber(t, files[i]) < migrationNumber(t, files[j]) })
	for _, file := range files {
		migration, err := os.ReadFile(file)
		require.NoError(t, err)
		up, _, ok := strings.Cut(string(migration), "-- +goose Down")
		require.True(t, ok, file)
		_, err = pool.ExecContext(ctx, up)
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

func newSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() { require.NoError(t, mock.ExpectationsWereMet()) })
	return db, mock
}
