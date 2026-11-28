package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type OnlineconfRepository struct {
	db DB
}

func NewOnlineconfRepository(db DB) *OnlineconfRepository {
	return &OnlineconfRepository{db: db}
}

func scanOnlineconfEntry(row scanner) (domain.OnlineconfEntry, error) {
	var entry domain.OnlineconfEntry
	err := row.Scan(&entry.ID, &entry.Scope, &entry.Name, &entry.Kind, &entry.Value)
	return entry, err
}

func onlineconfWriteError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrOnlineconfMissing
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrOnlineconfExists
		case "23514", "22001", "22P02", "22P05", "22003", "22021":
			return domain.ErrInvalidOnlineconf
		}
	}
	return err
}

func (r *OnlineconfRepository) Create(
	ctx context.Context, entry domain.OnlineconfEntry,
) (domain.OnlineconfEntry, error) {
	row := r.db.QueryRowContext(ctx, createConf, entry.Scope, entry.Name, entry.Kind, string(entry.Value))
	result, err := scanOnlineconfEntry(row)
	return result, onlineconfWriteError(err)
}

func (r *OnlineconfRepository) Update(
	ctx context.Context, id int, patch domain.OnlineconfPatch,
) (domain.OnlineconfEntry, error) {
	var value *string
	if patch.Value != nil {
		raw := string(patch.Value)
		value = &raw
	}

	row := r.db.QueryRowContext(ctx, updateConf, id, patch.Scope, patch.Name, value)
	result, err := scanOnlineconfEntry(row)
	return result, onlineconfWriteError(err)
}

func (r *OnlineconfRepository) Search(
	ctx context.Context, filter domain.OnlineconfFilter,
) ([]domain.OnlineconfEntry, error) {

	rows, err := r.db.QueryContext(ctx, searchConf, filter.Name, filter.Scope, filter.Kind)
	if err != nil {
		return nil, err
	}
	return collectRows(rows, scanOnlineconfEntry)
}
