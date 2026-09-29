package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type AdRepository struct {
	db *pgxpool.Pool
}

func NewAdRepository(db *pgxpool.Pool) *AdRepository {
	return &AdRepository{db: db}
}

type AdListFilter struct {
	Query       string
	CategoryID  *int32
	City        string
	PriceMin    string
	PriceMax    string
	HasDelivery *bool
	Sort        string
	Limit       int
	Offset      int
	ViewerID    *int32
}

const adFilterConditions = `
	a.status = 'active'
	AND ($1 = '' OR a.title ILIKE '%' || $1 || '%' OR a.description ILIKE '%' || $1 || '%')
	AND ($2::int IS NULL OR a.category_id = $2)
	AND ($3 = '' OR a.city = $3)
	AND ($4 = '' OR a.price >= $4::numeric)
	AND ($5 = '' OR a.price <= $5::numeric)
	AND ($6::bool IS NULL OR a.has_delivery = $6)`

func (r *AdRepository) List(ctx context.Context, f AdListFilter) ([]domain.AdCard, int, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	orderBy := "a.created_at DESC"
	switch f.Sort {
	case "price_asc":
		orderBy = "a.price ASC, a.created_at DESC"
	case "price_desc":
		orderBy = "a.price DESC, a.created_at DESC"
	}

	query := `
		SELECT
			a.id, a.title, a.price::text, a.city, a.has_delivery, a.category_id, a.created_at,
			(SELECT ai.storage_key FROM ad_image ai WHERE ai.ad_id = a.id ORDER BY ai.position LIMIT 1),
			rv.avg_rating, COALESCE(rv.reviews_count, 0),
			$7::int IS NOT NULL AND fav.ad_id IS NOT NULL
		FROM ad a
		LEFT JOIN (
			SELECT ad_id, AVG(rating)::float8 AS avg_rating, COUNT(*) AS reviews_count
			FROM review GROUP BY ad_id
		) rv ON rv.ad_id = a.id
		LEFT JOIN favorites fav ON fav.ad_id = a.id AND fav.user_id = $7
		WHERE` + adFilterConditions + `
		ORDER BY ` + orderBy + `
		LIMIT $8 OFFSET $9`

	rows, err := r.db.Query(ctx, query,
		f.Query, f.CategoryID, f.City, f.PriceMin, f.PriceMax, f.HasDelivery, f.ViewerID, f.Limit, f.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var ads []domain.AdCard
	for rows.Next() {
		var ad domain.AdCard
		if err := rows.Scan(
			&ad.ID, &ad.Title, &ad.Price, &ad.City, &ad.HasDelivery, &ad.CategoryID, &ad.CreatedAt,
			&ad.ImageURL, &ad.Rating, &ad.ReviewsCount, &ad.IsFavorite,
		); err != nil {
			return nil, 0, err
		}
		ads = append(ads, ad)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	countQuery := `SELECT count(*) FROM ad a WHERE` + adFilterConditions
	if err := r.db.QueryRow(ctx, countQuery,
		f.Query, f.CategoryID, f.City, f.PriceMin, f.PriceMax, f.HasDelivery,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return ads, total, nil
}
