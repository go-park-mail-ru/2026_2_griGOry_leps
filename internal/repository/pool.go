package repository

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.ConnConfig.Tracer = queryLogger{}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// queryLogger пишет каждый запрос к базе в лог с request_id из контекста.
// Аргументы запроса не логируются: среди них бывают хеши паролей.
type queryLogger struct{}

type queryStartKey struct{}

type queryStart struct {
	sql string
	at  time.Time
}

func (queryLogger) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryStartKey{}, queryStart{sql: data.SQL, at: time.Now()})
}

func (queryLogger) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	start, _ := ctx.Value(queryStartKey{}).(queryStart)
	attrs := []any{
		"sql", strings.Join(strings.Fields(start.sql), " "),
		"duration_ms", float64(time.Since(start.at).Microseconds()) / 1000,
		"rows", data.CommandTag.RowsAffected(),
	}
	if data.Err != nil {
		slog.WarnContext(ctx, "db query failed", append(attrs, "err", data.Err)...)
		return
	}
	slog.DebugContext(ctx, "db query", attrs...)
}
