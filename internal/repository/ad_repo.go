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

func adCardFields(ad *domain.Ad) []any {
	return []any{
		&ad.ID, &ad.UserID, &ad.CategoryID, &ad.Title, &ad.Description, &ad.Price, &ad.City,
		&ad.HasDelivery, &ad.IsFree, &ad.ImageURL, &ad.Status, &ad.CloseReason, &ad.CreatedAt, &ad.UpdatedAt,
	}
}

func scanAds(rows pgx.Rows) ([]domain.Ad, error) {
	defer rows.Close()
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

func readOnly(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin read tx: %w", err)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
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
	err := readOnly(ctx, r.pool, func(tx pgx.Tx) error {
		args := []any{f.Query, f.CategoryID, f.City, f.PriceMin, f.PriceMax, f.HasDelivery}
		if err := tx.QueryRow(ctx, countAdsQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count ads: %w", err)
		}

		rows, err := tx.Query(ctx, listAdsQuery, append(args, f.Sort, f.Limit, f.Offset)...)
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
	err := readOnly(ctx, r.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, countMyAdsQuery, f.UserID)
		if err != nil {
			return fmt.Errorf("count my ads: %w", err)
		}
		for rows.Next() {
			var (
				status string
				count  int
			)
			if err := rows.Scan(&status, &count); err != nil {
				rows.Close()
				return fmt.Errorf("scan my ads count: %w", err)
			}
			counts[status] = count
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("read my ads count: %w", err)
		}

		rows, err = tx.Query(ctx, listMyAdsQuery, f.UserID, f.Status, f.Limit, f.Offset)
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
