package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type CartRepository struct {
	db DB
}

func NewCartRepository(db DB) *CartRepository {
	return &CartRepository{db: db}
}

func scanCartEntries(rows pgx.Rows) ([]domain.CartEntry, error) {
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.CartEntry, error) {
		var e domain.CartEntry
		err := row.Scan(&e.ListingID, &e.Title, &e.Price, &e.ImageURL, &e.Address, &e.HasDelivery, &e.Status, &e.Reserved,
			&e.Seller.ID, &e.Seller.FirstName, &e.Seller.Avatar, &e.Seller.Rating)
		return e, err
	})
}

func (r *CartRepository) Entries(ctx context.Context, userID int32) ([]domain.CartEntry, error) {
	rows, err := conn(ctx, r.db).Query(ctx, getCartQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("get cart: %w", err)
	}
	entries, err := scanCartEntries(rows)
	if err != nil {
		return nil, fmt.Errorf("scan cart: %w", err)
	}
	return entries, nil
}

func (r *CartRepository) EntriesByIDs(ctx context.Context, userID int32, ids []int32) ([]domain.CartEntry, error) {
	rows, err := conn(ctx, r.db).Query(ctx, cartEntriesByIDsQuery, userID, ids)
	if err != nil {
		return nil, fmt.Errorf("get cart entries: %w", err)
	}
	entries, err := scanCartEntries(rows)
	if err != nil {
		return nil, fmt.Errorf("scan cart entries: %w", err)
	}
	return entries, nil
}

func (r *CartRepository) LockListings(ctx context.Context, ids []int32) error {
	rows, err := conn(ctx, r.db).Query(ctx, lockAdsQuery, ids)
	if err != nil {
		return fmt.Errorf("lock listings: %w", err)
	}
	rows.Close()
	return rows.Err()
}

func (r *CartRepository) ListingForCart(ctx context.Context, userID, listingID int32) (domain.CartListing, error) {
	var l domain.CartListing
	err := conn(ctx, r.db).QueryRow(ctx, getAdForCartQuery, userID, listingID).Scan(&l.SellerID, &l.Status, &l.Reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CartListing{}, domain.ErrListingNotFound
	}
	if err != nil {
		return domain.CartListing{}, fmt.Errorf("get listing for cart: %w", err)
	}
	return l, nil
}

func (r *CartRepository) Add(ctx context.Context, userID, listingID int32) error {
	db := conn(ctx, r.db)
	var cartID int32
	if err := db.QueryRow(ctx, ensureCartQuery, userID).Scan(&cartID); err != nil {
		return fmt.Errorf("ensure cart: %w", err)
	}
	if _, err := db.Exec(ctx, addCartItemQuery, cartID, listingID); err != nil {
		return fmt.Errorf("add cart item: %w", err)
	}
	return nil
}

func (r *CartRepository) Remove(ctx context.Context, userID int32, ids []int32) error {
	if _, err := conn(ctx, r.db).Exec(ctx, removeCartItemsQuery, userID, ids); err != nil {
		return fmt.Errorf("remove cart items: %w", err)
	}
	return nil
}

func (r *CartRepository) Count(ctx context.Context, userID int32) (int, error) {
	var n int
	if err := conn(ctx, r.db).QueryRow(ctx, countCartQuery, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count cart: %w", err)
	}
	return n, nil
}
