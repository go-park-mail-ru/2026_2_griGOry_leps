package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type PasswordResetRepository struct {
	db DB
}

func NewPasswordResetRepository(db DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *PasswordResetRepository) Create(ctx context.Context, tokenHash string, userID int32, expiresAt time.Time) error {
	_, err := conn(ctx, r.db).ExecContext(ctx, createPasswordResetQuery, tokenHash, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("create password reset: %w", err)
	}
	return nil
}

func (r *PasswordResetRepository) HasCreatedSince(ctx context.Context, userID int32, since time.Time) (bool, error) {
	var exists bool
	if err := conn(ctx, r.db).QueryRowContext(ctx, hasPasswordResetSinceQuery, userID, since).Scan(&exists); err != nil {
		return false, fmt.Errorf("check password resets: %w", err)
	}
	return exists, nil
}

func (r *PasswordResetRepository) Get(ctx context.Context, tokenHash string) (domain.PasswordReset, error) {
	return r.get(ctx, getPasswordResetQuery, tokenHash)
}

func (r *PasswordResetRepository) GetForUpdate(ctx context.Context, tokenHash string) (domain.PasswordReset, error) {
	return r.get(ctx, getPasswordResetForUpdateQuery, tokenHash)
}

func (r *PasswordResetRepository) get(ctx context.Context, query, tokenHash string) (domain.PasswordReset, error) {
	var reset domain.PasswordReset
	err := conn(ctx, r.db).QueryRowContext(ctx, query, tokenHash).Scan(&reset.UserID, &reset.ExpiresAt, &reset.UsedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PasswordReset{}, domain.ErrInvalidResetToken
	}
	if err != nil {
		return domain.PasswordReset{}, fmt.Errorf("get password reset: %w", err)
	}
	return reset, nil
}

func (r *PasswordResetRepository) UseAll(ctx context.Context, userID int32) error {
	_, err := conn(ctx, r.db).ExecContext(ctx, usePasswordResetsQuery, userID)
	if err != nil {
		return fmt.Errorf("use password resets: %w", err)
	}
	return nil
}
