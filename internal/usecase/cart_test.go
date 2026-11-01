package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	buyerID    int32 = 1
	sellerA    int32 = 10
	sellerB    int32 = 20
	strangerID int32 = 3
)

func newCartEnv() (*CartUsecase, *cartRepoFake) {
	repo := newCartRepoFake()
	repo.put(101, sellerA, 1500000, true)
	repo.put(102, sellerA, 230000, true)
	repo.put(201, sellerB, 1850000, false)
	return NewCartUsecase(repo, txFake{}), repo
}

func TestCart_AddRules(t *testing.T) {
	uc, repo := newCartEnv()
	ctx := context.Background()

	count, err := uc.Add(ctx, buyerID, 101)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	count, err = uc.Add(ctx, buyerID, 101)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	_, err = uc.Add(ctx, sellerA, 101)
	require.ErrorIs(t, err, domain.ErrOwnListing)

	_, err = uc.Add(ctx, buyerID, 999)
	require.ErrorIs(t, err, domain.ErrListingNotFound)

	e := repo.catalog[102]
	e.Status = domain.AdStatusDraft
	repo.catalog[102] = e
	_, err = uc.Add(ctx, buyerID, 102)
	require.ErrorIs(t, err, domain.ErrListingNotFound)

	e.Status = domain.AdStatusClosed
	repo.catalog[102] = e
	_, err = uc.Add(ctx, buyerID, 102)
	require.ErrorIs(t, err, domain.ErrListingUnavailable)
}

func TestCart_GroupsBySellerNewestFirst(t *testing.T) {
	uc, repo := newCartEnv()
	ctx := context.Background()
	for _, id := range []int32{101, 201, 102} {
		_, err := uc.Add(ctx, buyerID, id)
		require.NoError(t, err)
	}
	e := repo.catalog[201]
	e.Status = domain.AdStatusSold
	repo.catalog[201] = e

	cart, err := uc.Get(ctx, buyerID)
	require.NoError(t, err)
	require.Equal(t, 3, cart.ItemsCount)
	require.Len(t, cart.Groups, 2)
	require.Equal(t, sellerA, cart.Groups[0].Seller.ID)
	require.Equal(t, int32(102), cart.Groups[0].Items[0].ListingID)
	require.Equal(t, int32(101), cart.Groups[0].Items[1].ListingID)
	require.Equal(t, domain.UnavailableSold, cart.Groups[1].Items[0].UnavailableReason())
}

func TestCart_EmptyCartHasNoGroups(t *testing.T) {
	uc, _ := newCartEnv()
	cart, err := uc.Get(context.Background(), strangerID)
	require.NoError(t, err)
	require.Equal(t, 0, cart.ItemsCount)
	require.NotNil(t, cart.Groups)
}

func TestCart_BulkAddSkipsUnavailable(t *testing.T) {
	uc, repo := newCartEnv()
	e := repo.catalog[102]
	e.Status = domain.AdStatusClosed
	repo.catalog[102] = e

	count, skipped, err := uc.BulkAdd(context.Background(), buyerID, []int32{101, 102, 201, 999, 101})
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, []int32{102, 999}, skipped)
}

func TestCart_RemoveAndValidateIDs(t *testing.T) {
	uc, _ := newCartEnv()
	ctx := context.Background()
	_, _, err := uc.BulkAdd(ctx, buyerID, []int32{101, 102, 201})
	require.NoError(t, err)

	count, err := uc.Remove(ctx, buyerID, []int32{101, 201, 555})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	_, err = uc.Remove(ctx, buyerID, nil)
	require.ErrorIs(t, err, domain.ErrInvalidListingIDs)
	_, err = uc.Remove(ctx, buyerID, []int32{0})
	require.ErrorIs(t, err, domain.ErrInvalidListingIDs)
	_, _, err = uc.BulkAdd(ctx, buyerID, make([]int32, domain.MaxCartBatch+1))
	require.ErrorIs(t, err, domain.ErrInvalidListingIDs)
}
