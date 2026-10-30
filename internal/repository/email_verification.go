package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/jackc/pgx/v5"
)

type EmailVerificationRepository struct {
	db DB
}

func NewEmailVerificationRepository(db DB) *EmailVerificationRepository {
	return &EmailVerificationRepository{db: db}
}

func (r *EmailVerificationRepository) Create(ctx context.Context, tokenHash string, userID int32, email string, expiresAt time.Time) error {
	_, err := conn(ctx, r.db).Exec(ctx, createEmailVerificationQuery, tokenHash, userID, email, expiresAt)
	if err != nil {
		return fmt.Errorf("create email verification: %w", err)
	}
	return nil
}

func (r *EmailVerificationRepository) LastCreatedAt(ctx context.Context, userID int32) (time.Time, error) {
	var last *time.Time
	if err := conn(ctx, r.db).QueryRow(ctx, lastEmailVerificationQuery, userID).Scan(&last); err != nil {
		return time.Time{}, fmt.Errorf("last email verification: %w", err)
	}
	if last == nil {
		return time.Time{}, nil
	}
	return *last, nil
}

func (r *EmailVerificationRepository) Get(ctx context.Context, tokenHash string) (domain.EmailVerification, error) {
	return r.get(ctx, getEmailVerificationQuery, tokenHash)
}

func (r *EmailVerificationRepository) GetForUpdate(ctx context.Context, tokenHash string) (domain.EmailVerification, error) {
	return r.get(ctx, getEmailVerificationForUpdateQuery, tokenHash)
}

func (r *EmailVerificationRepository) get(ctx context.Context, query, tokenHash string) (domain.EmailVerification, error) {
	var v domain.EmailVerification
	err := conn(ctx, r.db).QueryRow(ctx, query, tokenHash).Scan(&v.UserID, &v.Email, &v.ExpiresAt, &v.UsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EmailVerification{}, domain.ErrInvalidVerifyToken
	}
	if err != nil {
		return domain.EmailVerification{}, fmt.Errorf("get email verification: %w", err)
	}
	return v, nil
}

func (r *EmailVerificationRepository) UseAll(ctx context.Context, userID int32) error {
	_, err := conn(ctx, r.db).Exec(ctx, useEmailVerificationsQuery, userID)
	if err != nil {
		return fmt.Errorf("use email verifications: %w", err)
	}
	return nil
}
