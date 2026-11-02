package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type AdRepository struct {
	pool *sql.DB
}

func NewAdRepository(pool *sql.DB) *AdRepository {
	return &AdRepository{pool: pool}
}

func adCardFields(ad *domain.Ad) []any {
	return []any{
		&ad.ID, &ad.UserID, &ad.CategoryID, &ad.Title, &ad.Description, &ad.Price, &ad.City,
		&ad.HasDelivery, &ad.IsFree, &ad.ImageURL, &ad.Status, &ad.CloseReason, &ad.CreatedAt, &ad.UpdatedAt,
	}
}

func scanAds(rows *sql.Rows) ([]domain.Ad, error) {
	defer func() { _ = rows.Close() }()
	ads := make([]domain.Ad, 0)
	for rows.Next() {
		var ad domain.Ad
		if err := rows.Scan(adCardFields(&ad)...); err != nil {
			return nil, fmt.Errorf("scan ad: %w", err)
		}
		ads = append(ads, ad)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read ad: %w", err)
	}
	return ads, nil
}

func readOnly(ctx context.Context, pool *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := pool.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin read tx: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit read tx: %w", err)
	}
	return nil
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

	var (
		ads   []domain.Ad
		total int
	)
	err := readOnly(ctx, r.pool, func(tx *sql.Tx) error {
		args := []any{f.Query, f.CategoryID, f.City, f.PriceMin, f.PriceMax, f.HasDelivery}
		if err := tx.QueryRowContext(ctx, countAdsQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count ads: %w", err)
		}

		rows, err := tx.QueryContext(ctx, listAdsQuery, append(args, f.Sort, f.Limit, f.Offset)...)
		if err != nil {
			return fmt.Errorf("list ads: %w", err)
		}
		ads, err = scanAds(rows)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	return ads, total, nil
}

func (r *AdRepository) ListByUser(ctx context.Context, f domain.MyAdFilter) ([]domain.Ad, map[string]int, error) {
	if f.Offset < 0 || f.Limit < 0 {
		return nil, nil, fmt.Errorf("negative pagination")
	}

	var (
		ads    []domain.Ad
		counts = make(map[string]int)
	)
	err := readOnly(ctx, r.pool, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, countMyAdsQuery, f.UserID)
		if err != nil {
			return fmt.Errorf("count my ads: %w", err)
		}
		for rows.Next() {
			var (
				status string
				count  int
			)
			if err := rows.Scan(&status, &count); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan my ads count: %w", err)
			}
			counts[status] = count
		}
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("read my ads count: %w", err)
		}

		rows, err = tx.QueryContext(ctx, listMyAdsQuery, f.UserID, f.Status, f.Limit, f.Offset)
		if err != nil {
			return fmt.Errorf("list my ads: %w", err)
		}
		ads, err = scanAds(rows)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return ads, counts, nil
}
