package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type FavoritesRepository struct {
	db DB
}

func NewFavoritesRepository(db DB) *FavoritesRepository {
	return &FavoritesRepository{db: db}
}

func (r *FavoritesRepository) List(ctx context.Context, userID int32) ([]domain.Ad, error) {
	const query = `
		SELECT ` + adCardColumns + `
		FROM favorites f
		JOIN ad a ON a.id = f.ad_id
		` + adMainImageJoin + `
		WHERE f.user_id = $1
		  AND a.status IN ('active', 'closed', 'sold')
		ORDER BY f.id DESC
	`

	rows, err := conn(ctx, r.db).QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}

	return scanAds(rows)
}

func (r *FavoritesRepository) Remove(ctx context.Context, userID, listingID int32) error {
	const query = `
		DELETE FROM favorites
		WHERE user_id = $1 AND ad_id = $2
	`

	_, err := conn(ctx, r.db).ExecContext(ctx, query, userID, listingID)
	if err != nil {
		return fmt.Errorf("remove favorite: %w", err)
	}

	return nil
}

func (r *FavoritesRepository) Add(ctx context.Context, userID, listingID int32) error {
	const query = `
		WITH listing AS (
			SELECT id, status
			FROM ad
			WHERE id = $2
			  AND status IN ('active', 'closed', 'sold')
			FOR SHARE
		),
		added AS (
			INSERT INTO favorites (user_id, ad_id)
			SELECT $1, id
			FROM listing
			WHERE status = 'active'
			ON CONFLICT (user_id, ad_id) DO NOTHING
		)
		SELECT status = 'active' OR EXISTS (
			SELECT 1
			FROM favorites
			WHERE user_id = $1 AND ad_id = $2
		)
		FROM listing
	`

	var available bool
	err := conn(ctx, r.db).QueryRowContext(ctx, query, userID, listingID).Scan(&available)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrListingNotFound
	}
	if err != nil {
		return fmt.Errorf("add favorite: %w", err)
	}
	if !available {
		return domain.ErrListingUnavailable
	}

	return nil
}
