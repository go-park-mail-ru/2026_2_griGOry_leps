package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, id string, userID int32, expiresAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		id, userID, expiresAt,
	)
	return err
}

func (r *SessionRepository) GetByID(ctx context.Context, id string) (domain.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	var session domain.Session

	row := r.db.QueryRow(ctx,
		`SELECT id, user_id, expires_at FROM sessions WHERE id = $1`,
		id,
	)

	err := row.Scan(&session.ID, &session.UserID, &session.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, ErrSessionNotFound
		}
		return domain.Session{}, err
	}

	return session, nil
}

func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}
