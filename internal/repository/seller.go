package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SellerRepository struct {
	pool *pgxpool.Pool
}

func NewSellerRepository(pool *pgxpool.Pool) *SellerRepository {
	return &SellerRepository{pool: pool}
}

func (r *SellerRepository) Get(ctx context.Context, id int32) (domain.SellerProfile, error) {
	var s domain.SellerProfile
	err := r.pool.QueryRow(ctx, getSellerQuery, id).Scan(
		&s.ID, &s.FirstName, &s.Avatar, &s.Rating, &s.ReviewsCount, &s.CreatedAt,
		&s.Location, &s.PhoneVerified, &s.CompletedSalesCount, &s.ActiveListingsCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SellerProfile{}, domain.ErrSellerNotFound
	}
	if err != nil {
		return domain.SellerProfile{}, fmt.Errorf("get seller: %w", err)
	}
	return s, nil
}

func ensureSeller(ctx context.Context, tx pgx.Tx, id int32) error {
	var exists bool
	if err := tx.QueryRow(ctx, sellerExistsQuery, id).Scan(&exists); err != nil {
		return fmt.Errorf("check seller: %w", err)
	}
	if !exists {
		return domain.ErrSellerNotFound
	}
	return nil
}

func (r *SellerRepository) ListAds(ctx context.Context, f domain.SellerAdFilter) (domain.SellerAdPage, error) {
	if f.Offset < 0 || f.Limit < 0 {
		return domain.SellerAdPage{}, fmt.Errorf("negative pagination")
	}

	page := domain.SellerAdPage{AdPage: domain.AdPage{Limit: f.Limit, Offset: f.Offset}}
	err := readOnly(ctx, r.pool, func(tx pgx.Tx) error {
		if err := ensureSeller(ctx, tx, f.SellerID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, countSellerAdsQuery, f.SellerID, f.CategoryID).Scan(&page.Total); err != nil {
			return fmt.Errorf("count seller ads: %w", err)
		}

		rows, err := tx.Query(ctx, listSellerAdsQuery, f.SellerID, f.CategoryID, f.Sort, f.Limit, f.Offset)
		if err != nil {
			return fmt.Errorf("list seller ads: %w", err)
		}
		if page.Items, err = scanAds(rows); err != nil {
			return err
		}

		rows, err = tx.Query(ctx, listSellerCategoriesQuery, f.SellerID)
		if err != nil {
			return fmt.Errorf("list seller categories: %w", err)
		}
		defer rows.Close()
		page.Categories = make([]domain.SellerCategory, 0)
		for rows.Next() {
			var c domain.SellerCategory
			if err := rows.Scan(&c.ID, &c.Name, &c.Count); err != nil {
				return fmt.Errorf("scan seller category: %w", err)
			}
			page.Categories = append(page.Categories, c)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("read seller categories: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.SellerAdPage{}, err
	}
	return page, nil
}

func (r *SellerRepository) ListReviews(ctx context.Context, f domain.ReviewFilter) (domain.ReviewPage, error) {
	if f.Offset < 0 || f.Limit < 0 {
		return domain.ReviewPage{}, fmt.Errorf("negative pagination")
	}

	page := domain.ReviewPage{Limit: f.Limit, Offset: f.Offset}
	err := readOnly(ctx, r.pool, func(tx pgx.Tx) error {
		if err := ensureSeller(ctx, tx, f.SellerID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, countSellerReviewsQuery, f.SellerID, f.WithPhotos).Scan(&page.Total); err != nil {
			return fmt.Errorf("count seller reviews: %w", err)
		}

		var err error
		if page.Items, err = listReviews(ctx, tx, f); err != nil {
			return err
		}
		page.ReviewSummary, err = reviewSummary(ctx, tx, f.SellerID)
		return err
	})
	if err != nil {
		return domain.ReviewPage{}, err
	}
	return page, nil
}

func listReviews(ctx context.Context, tx pgx.Tx, f domain.ReviewFilter) ([]domain.Review, error) {
	rows, err := tx.Query(ctx, listSellerReviewsQuery, f.SellerID, f.WithPhotos, f.Sort, f.Limit, f.Offset)
	if err != nil {
		return nil, fmt.Errorf("list seller reviews: %w", err)
	}
	defer rows.Close()

	reviews := make([]domain.Review, 0)
	for rows.Next() {
		var rv domain.Review
		err := rows.Scan(&rv.ID, &rv.Author.ID, &rv.Author.FirstName, &rv.Author.Avatar,
			&rv.Rating, &rv.Text, &rv.ListingTitle, &rv.CreatedAt, &rv.Photos)
		if err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		reviews = append(reviews, rv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read reviews: %w", err)
	}
	return reviews, nil
}

func reviewSummary(ctx context.Context, tx pgx.Tx, sellerID int32) (domain.ReviewSummary, error) {
	summary := domain.ReviewSummary{Distribution: map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0}}
	if err := tx.QueryRow(ctx, sellerRatingQuery, sellerID).Scan(&summary.Rating); err != nil {
		return domain.ReviewSummary{}, fmt.Errorf("get seller rating: %w", err)
	}

	rows, err := tx.Query(ctx, sellerRatingDistributionQuery, sellerID)
	if err != nil {
		return domain.ReviewSummary{}, fmt.Errorf("get rating distribution: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var stars, count int
		if err := rows.Scan(&stars, &count); err != nil {
			return domain.ReviewSummary{}, fmt.Errorf("scan rating distribution: %w", err)
		}
		summary.Distribution[stars] = count
	}
	if err := rows.Err(); err != nil {
		return domain.ReviewSummary{}, fmt.Errorf("read rating distribution: %w", err)
	}
	return summary, nil
}

func (r *SellerRepository) CreateReport(ctx context.Context, report domain.SellerReport) error {
	_, err := r.pool.Exec(ctx, createSellerReportQuery, report.SellerID, report.ReporterID, report.Reason, report.Comment)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "seller_report_seller_id_fkey" {
		return domain.ErrSellerNotFound
	}
	if err != nil {
		return fmt.Errorf("create seller report: %w", err)
	}
	return nil
}
