package usecase

import (
	"context"
	"errors"
	"slices"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type CartUsecase struct {
	carts  CartRepository
	tx     Transactor
	prices domain.DeliveryPrices
}

func NewCartUsecase(carts CartRepository, tx Transactor, prices domain.DeliveryPrices) *CartUsecase {
	return &CartUsecase{carts: carts, tx: tx, prices: prices}
}

func (uc *CartUsecase) Get(ctx context.Context, userID int32) (domain.Cart, error) {
	entries, err := uc.carts.Entries(ctx, userID)
	if err != nil {
		return domain.Cart{}, err
	}
	cheapest := uc.prices.Cheapest()
	for i := range entries {
		if entries[i].HasDelivery && entries[i].UnavailableReason() == "" {
			entries[i].DeliveryFrom = &cheapest
		}
	}
	return groupCart(entries), nil
}

func groupCart(entries []domain.CartEntry) domain.Cart {
	cart := domain.Cart{ItemsCount: len(entries), Groups: []domain.CartGroup{}}
	index := make(map[int32]int)
	for _, e := range entries {
		i, ok := index[e.Seller.ID]
		if !ok {
			i = len(cart.Groups)
			index[e.Seller.ID] = i
			cart.Groups = append(cart.Groups, domain.CartGroup{Seller: e.Seller})
		}
		cart.Groups[i].Items = append(cart.Groups[i].Items, e)
	}
	return cart
}

func (uc *CartUsecase) Add(ctx context.Context, userID, listingID int32) (int, error) {
	var count int
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.add(ctx, userID, listingID); err != nil {
			return err
		}
		var err error
		count, err = uc.carts.Count(ctx, userID)
		return err
	})
	return count, err
}

func (uc *CartUsecase) add(ctx context.Context, userID, listingID int32) error {
	l, err := uc.carts.ListingForCart(ctx, listingID)
	if err != nil {
		return err
	}
	if l.SellerID == userID {
		return domain.ErrOwnListing
	}
	switch l.Status {
	case domain.AdStatusDraft, domain.AdStatusModeration, domain.AdStatusRejected:
		return domain.ErrListingNotFound
	case domain.AdStatusActive:
	default:
		return domain.ErrListingUnavailable
	}
	return uc.carts.Add(ctx, userID, listingID, l.SellerID)
}

func (uc *CartUsecase) BulkAdd(ctx context.Context, userID int32, ids []int32) (int, []int32, error) {
	ids, err := checkListingIDs(ids)
	if err != nil {
		return 0, nil, err
	}
	slices.Sort(ids)

	var (
		count   int
		skipped = []int32{}
	)
	err = uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		for _, id := range ids {
			err := uc.add(ctx, userID, id)
			switch {
			case err == nil:
			case errors.Is(err, domain.ErrListingNotFound), errors.Is(err, domain.ErrOwnListing),
				errors.Is(err, domain.ErrListingUnavailable):
				skipped = append(skipped, id)
			default:
				return err
			}
		}
		var err error
		count, err = uc.carts.Count(ctx, userID)
		return err
	})
	if err != nil {
		return 0, nil, err
	}
	return count, skipped, nil
}

func (uc *CartUsecase) Remove(ctx context.Context, userID int32, ids []int32) (int, error) {
	ids, err := checkListingIDs(ids)
	if err != nil {
		return 0, err
	}
	if err := uc.carts.Remove(ctx, userID, ids); err != nil {
		return 0, err
	}
	return uc.carts.Count(ctx, userID)
}

func checkListingIDs(ids []int32) ([]int32, error) {
	if len(ids) == 0 || len(ids) > domain.MaxCartBatch {
		return nil, domain.ErrInvalidListingIDs
	}
	seen := make(map[int32]bool, len(ids))
	unique := make([]int32, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, domain.ErrInvalidListingIDs
		}
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	return unique, nil
}
