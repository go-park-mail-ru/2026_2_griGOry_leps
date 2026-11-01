package usecase

import (
	"context"
	"slices"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type cartRepoFake struct {
	catalog map[int32]domain.CartEntry
	carts   map[int32][]int32
}

func newCartRepoFake() *cartRepoFake {
	return &cartRepoFake{catalog: make(map[int32]domain.CartEntry), carts: make(map[int32][]int32)}
}

func (r *cartRepoFake) put(id, sellerID int32, price int64, hasDelivery bool) {
	address := "Москва, м. Сокол"
	r.catalog[id] = domain.CartEntry{
		ListingID: id, Title: "Товар", Price: &price, Address: &address, HasDelivery: hasDelivery,
		Status: domain.AdStatusActive, Seller: domain.CartSeller{ID: sellerID, FirstName: "Продавец"},
	}
}

func (r *cartRepoFake) Entries(_ context.Context, userID int32) ([]domain.CartEntry, error) {
	ids := r.carts[userID]
	entries := make([]domain.CartEntry, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		entries = append(entries, r.catalog[ids[i]])
	}
	return entries, nil
}

func (r *cartRepoFake) ListingForCart(_ context.Context, listingID int32) (domain.CartListing, error) {
	e, ok := r.catalog[listingID]
	if !ok {
		return domain.CartListing{}, domain.ErrListingNotFound
	}
	return domain.CartListing{SellerID: e.Seller.ID, Status: e.Status}, nil
}

func (r *cartRepoFake) Add(_ context.Context, userID, listingID, _ int32) error {
	if !slices.Contains(r.carts[userID], listingID) {
		r.carts[userID] = append(r.carts[userID], listingID)
	}
	return nil
}

func (r *cartRepoFake) Remove(_ context.Context, userID int32, ids []int32) error {
	r.carts[userID] = slices.DeleteFunc(r.carts[userID], func(id int32) bool { return slices.Contains(ids, id) })
	return nil
}

func (r *cartRepoFake) Count(_ context.Context, userID int32) (int, error) {
	return len(r.carts[userID]), nil
}
