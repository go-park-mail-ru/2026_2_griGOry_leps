package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type ListingRepository struct {
	db DB
}

func NewListingRepository(db DB) *ListingRepository {
	return &ListingRepository{db: db}
}

func (r *ListingRepository) Get(ctx context.Context, id, viewerID int32) (domain.Listing, error) {
	db := conn(ctx, r.db)

	var (
		l          domain.Listing
		categoryID *int32
		catName    string
		catPath    string
	)
	err := db.QueryRow(ctx, getListingQuery, id, viewerID).Scan(
		&l.ID, &l.Seller.ID, &categoryID, &catName, &catPath,
		&l.Title, &l.Description, &l.Price, &l.IsFree, &l.IsNegotiable, &l.Condition,
		&l.City, &l.Address, &l.HasDelivery, &l.Status, &l.CloseReason,
		&l.PublishedAt, &l.CreatedAt, &l.UpdatedAt,
		&l.Seller.FirstName, &l.Seller.Avatar, &l.Seller.CreatedAt,
		&l.Seller.Rating, &l.Seller.ReviewsCount,
		&l.InCart,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Listing{}, domain.ErrListingNotFound
	}
	if err != nil {
		return domain.Listing{}, fmt.Errorf("get listing: %w", err)
	}

	if categoryID != nil {
		l.Category = &domain.ListingCategory{ID: *categoryID, Name: catName, Path: catPath}
	}

	rows, err := db.Query(ctx, listingPhotosQuery, id)
	if err != nil {
		return domain.Listing{}, fmt.Errorf("get listing photos: %w", err)
	}
	l.Photos, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ListingPhoto, error) {
		var p domain.ListingPhoto
		err := row.Scan(&p.ID, &p.URL)
		return p, err
	})
	if err != nil {
		return domain.Listing{}, fmt.Errorf("scan listing photos: %w", err)
	}

	return l, nil
}

func (r *ListingRepository) GetStateForUpdate(ctx context.Context, id int32) (domain.ListingState, error) {
	db := conn(ctx, r.db)

	var s domain.ListingState
	err := db.QueryRow(ctx, getListingStateForUpdateQuery, id).Scan(
		&s.UserID, &s.Status, &s.CategoryID, &s.Title, &s.Description, &s.Price,
		&s.IsFree, &s.IsNegotiable, &s.Condition, &s.Address, &s.HasDelivery,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ListingState{}, domain.ErrListingNotFound
	}
	if err != nil {
		return domain.ListingState{}, fmt.Errorf("lock listing: %w", err)
	}

	rows, err := db.Query(ctx, listingPhotosQuery, id)
	if err != nil {
		return domain.ListingState{}, fmt.Errorf("get listing photos: %w", err)
	}
	s.PhotoIDs, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (int32, error) {
		var photoID int32
		var url string
		err := row.Scan(&photoID, &url)
		return photoID, err
	})
	if err != nil {
		return domain.ListingState{}, fmt.Errorf("scan listing photos: %w", err)
	}
	return s, nil
}

func (r *ListingRepository) Create(ctx context.Context, s domain.ListingState) (int32, error) {
	var id int32
	err := conn(ctx, r.db).QueryRow(ctx, createListingQuery,
		s.UserID, s.Status, s.CategoryID, s.Title, s.Description, s.Price,
		s.IsFree, s.IsNegotiable, s.Condition, s.Address, s.HasDelivery,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create listing: %w", err)
	}
	return id, nil
}

func (r *ListingRepository) Update(ctx context.Context, id int32, s domain.ListingState) error {
	_, err := conn(ctx, r.db).Exec(ctx, updateListingQuery, id,
		s.Status, s.CategoryID, s.Title, s.Description, s.Price,
		s.IsFree, s.IsNegotiable, s.Condition, s.Address, s.HasDelivery,
	)
	if err != nil {
		return fmt.Errorf("update listing: %w", err)
	}
	return nil
}

func (r *ListingRepository) SetPhotos(ctx context.Context, userID, listingID int32, photoIDs []int32) ([]string, error) {
	db := conn(ctx, r.db)
	if photoIDs == nil {
		photoIDs = []int32{}
	}

	rows, err := db.Query(ctx, lockOwnPhotosQuery, photoIDs, userID, listingID)
	if err != nil {
		return nil, fmt.Errorf("lock photos: %w", err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return nil, fmt.Errorf("scan photos: %w", err)
	}
	if len(found) != len(photoIDs) {
		return nil, domain.ErrInvalidPhotoID
	}

	rows, err = db.Query(ctx, detachMissingPhotosQuery, listingID, photoIDs)
	if err != nil {
		return nil, fmt.Errorf("detach photos: %w", err)
	}
	removed, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("scan detached photos: %w", err)
	}

	if _, err := db.Exec(ctx, shiftPhotoPositionsQuery, listingID); err != nil {
		return nil, fmt.Errorf("shift photos: %w", err)
	}
	if _, err := db.Exec(ctx, attachPhotosQuery, listingID, photoIDs); err != nil {
		return nil, fmt.Errorf("attach photos: %w", err)
	}
	return removed, nil
}

func (r *ListingRepository) Delete(ctx context.Context, id int32) ([]string, error) {
	db := conn(ctx, r.db)

	rows, err := db.Query(ctx, deleteListingPhotosQuery, id)
	if err != nil {
		return nil, fmt.Errorf("get listing photos: %w", err)
	}
	urls, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("scan listing photos: %w", err)
	}

	if _, err := db.Exec(ctx, deleteListingQuery, id); err != nil {
		return nil, fmt.Errorf("delete listing: %w", err)
	}
	return urls, nil
}

func (r *ListingRepository) Close(ctx context.Context, id int32, reason string) error {
	if _, err := conn(ctx, r.db).Exec(ctx, closeListingQuery, id, reason); err != nil {
		return fmt.Errorf("close listing: %w", err)
	}
	return nil
}

func (r *ListingRepository) IsLeafCategory(ctx context.Context, id int32) (bool, error) {
	var ok bool
	if err := conn(ctx, r.db).QueryRow(ctx, isLeafCategoryQuery, id).Scan(&ok); err != nil {
		return false, fmt.Errorf("check category: %w", err)
	}
	return ok, nil
}

func (r *ListingRepository) ListCategories(ctx context.Context) ([]domain.Category, error) {
	rows, err := conn(ctx, r.db).Query(ctx, listCategoriesQuery)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	categories, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Category, error) {
		var c domain.Category
		err := row.Scan(&c.ID, &c.ParentID, &c.Name, &c.Slug)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan categories: %w", err)
	}
	return categories, nil
}

func (r *ListingRepository) CreatePhoto(ctx context.Context, userID int32, url string) (int32, error) {
	var id int32
	if err := conn(ctx, r.db).QueryRow(ctx, createPhotoQuery, userID, url).Scan(&id); err != nil {
		return 0, fmt.Errorf("create photo: %w", err)
	}
	return id, nil
}

func (r *ListingRepository) ApproveModeration(ctx context.Context) (int64, error) {
	tag, err := conn(ctx, r.db).Exec(ctx, approveModerationQuery)
	if err != nil {
		return 0, fmt.Errorf("approve listings: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *ListingRepository) DeleteStalePhotos(ctx context.Context, before time.Time) ([]string, error) {
	rows, err := conn(ctx, r.db).Query(ctx, deleteStalePhotosQuery, before)
	if err != nil {
		return nil, fmt.Errorf("delete stale photos: %w", err)
	}
	urls, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("scan stale photos: %w", err)
	}
	return urls, nil
}
