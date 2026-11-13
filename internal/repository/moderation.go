package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type ModerationRepository struct{ db DB }

func NewModerationRepository(db DB) *ModerationRepository { return &ModerationRepository{db: db} }

const pendingListingsQuery = `
    SELECT id, title, moderation_revision, updated_at FROM ad
    WHERE status = 'moderation' AND id > $1
    ORDER BY id LIMIT $2
`

func (r *ModerationRepository) ListPending(ctx context.Context, afterID int32, limit int) ([]domain.ModerationItem, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, pendingListingsQuery, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list moderation queue: %w", err)
	}
	return collectRows(rows, func(row scanner) (domain.ModerationItem, error) {
		var item domain.ModerationItem
		err := row.Scan(&item.ID, &item.Title, &item.Revision, &item.UpdatedAt)
		return item, err
	})
}

const lockModerationRevisionQuery = `SELECT moderation_revision FROM ad WHERE id = $1 FOR UPDATE`

func (r *ModerationRepository) LockRevision(ctx context.Context, id int32) (int64, error) {
	var revision int64
	err := conn(ctx, r.db).QueryRowContext(ctx, lockModerationRevisionQuery, id).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrListingNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("lock moderation revision: %w", err)
	}
	return revision, nil
}

const decideModerationQuery = `
    WITH decided AS (
        UPDATE ad SET status = $4::text, rejection_reason = NULLIF($5::text, ''),
            published_at = CASE WHEN $4::text = 'active' THEN now() ELSE published_at END
        WHERE id = $1 AND status = 'moderation' AND moderation_revision = $3
        RETURNING id
    )
    INSERT INTO moderation_decision (ad_id, moderator_id, revision, status, reason)
    SELECT id, $2, $3, $4::text, NULLIF($5::text, '') FROM decided
`

func (r *ModerationRepository) Decide(ctx context.Context, id, moderatorID int32, revision int64, status, reason string) error {
	result, err := conn(ctx, r.db).ExecContext(ctx, decideModerationQuery, id, moderatorID, revision, status, reason)
	if err != nil {
		return fmt.Errorf("moderate listing: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("moderation rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrModerationConflict
	}
	return nil
}
