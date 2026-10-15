package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdRepository struct {
	pool *pgxpool.Pool
}

func NewAdRepository(pool *pgxpool.Pool) *AdRepository {
	return &AdRepository{pool: pool}
}

func (r *AdRepository) List(ctx context.Context, f domain.AdFilter) ([]domain.Ad, int, error) {
	switch {
	case f.Offset < 0 || f.Limit < 0:
		return nil, 0, fmt.Errorf("negative pagination")
	case f.Sort == "":
		f.Sort = domain.AdSortNewest
	case f.Sort == domain.AdSortPriceAsc || f.Sort == domain.AdSortPriceDesc || f.Sort == domain.AdSortNewest:
	default:
		return nil, 0, domain.ErrInvalidSort
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, fmt.Errorf("begin list ads: %w", err)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	args := []any{f.Query, f.CategoryID, f.City, f.PriceMin, f.PriceMax, f.HasDelivery}
	var total int

	err = tx.QueryRow(ctx, countAdsQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count ads: %w", err)
	}

	args = append(args, f.Sort, f.Limit, f.Offset)
	rows, err := tx.Query(ctx, listAdsQuery, args...)

	if err != nil {
		return nil, 0, fmt.Errorf("count list: %w", err)
	}

	defer rows.Close()
	ads := make([]domain.Ad, 0)
	for rows.Next() {
		var ad domain.Ad
		err := rows.Scan(&ad.ID, &ad.UserID, &ad.CategoryID, &ad.Title, &ad.Description, &ad.Price, &ad.City, &ad.HasDelivery, &ad.ImageURL, &ad.Status, &ad.CreatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("scan ad: %w", err)
		}
		ads = append(ads, ad)
	}

	err = rows.Err()
	if err != nil {
		return nil, 0, fmt.Errorf("read ad: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("commit list ads: %w", err)
	}
	return ads, total, nil

}
