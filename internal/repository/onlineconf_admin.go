package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type OnlineConfAdminRepository struct {
	db DB
}

func NewOnlineConfAdminRepository(db DB) *OnlineConfAdminRepository {
	return &OnlineConfAdminRepository{db: db}
}

func (r *OnlineConfAdminRepository) CheckReady(ctx context.Context) error {
	if _, err := conn(ctx, r.db).ExecContext(ctx, checkOnlineConfSchemaQuery); err != nil {
		return fmt.Errorf("check onlineconf schema: %w", err)
	}
	return nil
}

func (r *OnlineConfAdminRepository) IsAdmin(ctx context.Context, userID int32) (bool, error) {
	var exists bool
	if err := conn(ctx, r.db).QueryRowContext(ctx, isOnlineConfAdminQuery, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check onlineconf admin: %w", err)
	}
	return exists, nil
}

func (r *OnlineConfAdminRepository) Grant(ctx context.Context, userID int32, grantedBy *int32) (bool, error) {
	result, err := conn(ctx, r.db).ExecContext(ctx, grantOnlineConfAdminQuery, userID, grantedBy)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return false, domain.ErrUserNotFound
	}
	if err != nil {
		return false, fmt.Errorf("grant onlineconf admin: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("grant onlineconf admin: %w", err)
	}
	return affected == 1, nil
}
