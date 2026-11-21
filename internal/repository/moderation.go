package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type ModerationRepository struct{ db DB }

func NewModerationRepository(db DB) *ModerationRepository {
	return &ModerationRepository{db: db}
}

func (r *ModerationRepository) CheckRole(ctx context.Context, userID int32) error {
	var moderator bool
	err := conn(ctx, r.db).QueryRowContext(ctx, `
		SELECT is_moderator FROM user_account WHERE user_id = $1 FOR SHARE
	`, userID).Scan(&moderator)
	if errors.Is(err, sql.ErrNoRows) || err == nil && !moderator {
		return domain.ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("check moderator role: %w", err)
	}
	return nil
}

const listModerationQuery = `
	SELECT id, title, price, moderation_requested_at
	FROM ad
	WHERE status = 'moderation' AND user_id <> $1
	  AND ($2::timestamptz IS NULL OR (moderation_requested_at, id) > ($2::timestamptz, $3))
	ORDER BY moderation_requested_at, id
	LIMIT $4
`

func (r *ModerationRepository) List(ctx context.Context, actorID int32, after *time.Time, afterID int32, limit int) ([]domain.ModerationRequest, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, listModerationQuery, actorID, after, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list moderation queue: %w", err)
	}
	return collectRows(rows, func(row scanner) (domain.ModerationRequest, error) {
		var item domain.ModerationRequest
		err := row.Scan(&item.ID, &item.Title, &item.Price, &item.RequestedAt)
		return item, err
	})
}

func (r *ModerationRepository) LockPending(ctx context.Context, actorID, id int32) (int64, time.Time, error) {
	var revision int64
	var requestedAt *time.Time
	var status string
	var ownerID int32
	err := conn(ctx, r.db).QueryRowContext(ctx, `
		SELECT moderation_revision, moderation_requested_at, status, user_id FROM ad WHERE id = $1 FOR UPDATE
	`, id).Scan(&revision, &requestedAt, &status, &ownerID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, time.Time{}, domain.ErrListingNotFound
	case err != nil:
		return 0, time.Time{}, fmt.Errorf("lock moderation request: %w", err)
	case ownerID == actorID:
		return 0, time.Time{}, domain.ErrOwnModeration
	case status != domain.AdStatusModeration || requestedAt == nil:
		return 0, time.Time{}, domain.ErrModerationConflict
	}
	return revision, *requestedAt, nil
}

const decideModerationQuery = `
	WITH decided AS (
		UPDATE ad SET status = $4::varchar, rejection_reason = NULLIF($5::text, ''),
			published_at = CASE WHEN $4::varchar = 'active' THEN COALESCE(published_at, now()) ELSE published_at END
		WHERE id = $1 AND user_id <> $2 AND status = 'moderation' AND moderation_revision = $3
		RETURNING id
	)
	INSERT INTO moderation_decision (ad_id, moderator_id, revision, status, reason)
	SELECT id, $2, $3, $4::varchar, NULLIF($5::text, '') FROM decided
`

func (r *ModerationRepository) Decide(ctx context.Context, actorID, id int32, revision int64, status, reason string) error {
	result, err := conn(ctx, r.db).ExecContext(ctx, decideModerationQuery, id, actorID, revision, status, reason)
	if err != nil {
		return fmt.Errorf("moderate listing: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("moderation rows affected: %w", err)
	}
	if count == 0 {
		return domain.ErrModerationConflict
	}
	return nil
}
