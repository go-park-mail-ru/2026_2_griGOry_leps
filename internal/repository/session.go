package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type SessionRepository struct {
	db DB
}

func NewSessionRepository(db DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, id string, userID int32, expiresAt time.Time) error {
	_, err := conn(ctx, r.db).ExecContext(ctx, createSessionQuery, id, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *SessionRepository) GetByID(ctx context.Context, id string) (domain.Session, error) {
	var session domain.Session
	err := conn(ctx, r.db).QueryRowContext(ctx, getSessionQuery, id).Scan(&session.ID, &session.UserID, &session.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("get session: %w", err)
	}
	return session, nil
}

// Delete ничего не возвращает, если сессии уже нет: выход идемпотентный.
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	_, err := conn(ctx, r.db).ExecContext(ctx, deleteSessionQuery, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteOthers(ctx context.Context, userID int32, keepID string) error {
	_, err := conn(ctx, r.db).ExecContext(ctx, deleteOtherSessionsQuery, userID, keepID)
	if err != nil {
		return fmt.Errorf("delete other sessions: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteAll(ctx context.Context, userID int32) error {
	_, err := conn(ctx, r.db).ExecContext(ctx, deleteUserSessionsQuery, userID)
	if err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}
	return nil
}
