package repository

import (
	"context"
	"database/sql"
)

type DB interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type scanner interface {
	Scan(dest ...any) error
}
