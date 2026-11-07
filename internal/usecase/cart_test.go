package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	buyerID    int32 = 1
	sellerA    int32 = 10
	sellerB    int32 = 20
	strangerID int32 = 3
)

var testPrices = domain.DeliveryPrices{Courier: 39000, PickupPoint: 19000}

type cartTestEnv struct {
	uc   *CartUsecase
	repo *MockCartRepository
	tx   *MockTransactor
}

func newCartTestEnv(t *testing.T) *cartTestEnv {
	t.Helper()
	c := gomock.NewController(t)
	e := &cartTestEnv{repo: NewMockCartRepository(c), tx: NewMockTransactor(c)}
	e.uc = NewCartUsecase(e.repo, e.tx, testPrices)
	return e
}

func cartEntry(id, seller int32, price int64, delivery bool) domain.CartEntry {
	return domain.CartEntry{ListingID: id, Title: "item", Price: &price, Address: ptr("Moscow"), HasDelivery: delivery, Status: domain.AdStatusActive, Seller: domain.CartSeller{ID: seller, FirstName: "seller"}}
}

func TestCart_AddRules(t *testing.T) {
	for _, tc := range []struct {
		name          string
		user, id      int32
		listing       domain.CartListing
		repoErr, want error
	}{
		{"active", buyerID, 101, domain.CartListing{SellerID: sellerA, Status: domain.AdStatusActive}, nil, nil},
		{"own", sellerA, 101, domain.CartListing{SellerID: sellerA, Status: domain.AdStatusActive}, nil, domain.ErrOwnListing},
		{"missing", buyerID, 999, domain.CartListing{}, domain.ErrListingNotFound, domain.ErrListingNotFound},
		{"draft", buyerID, 102, domain.CartListing{SellerID: sellerA, Status: domain.AdStatusDraft}, nil, domain.ErrListingNotFound},
		{"closed", buyerID, 102, domain.CartListing{SellerID: sellerA, Status: domain.AdStatusClosed}, nil, domain.ErrListingUnavailable},
		{"reserved", buyerID, 102, domain.CartListing{SellerID: sellerA, Status: domain.AdStatusActive, Reserved: true}, nil, domain.ErrListingUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newCartTestEnv(t)
			expectTransaction(e.tx)
			e.repo.EXPECT().ListingForCart(gomock.Any(), tc.user, tc.id).Return(tc.listing, tc.repoErr)
			if tc.want == nil {
				e.repo.EXPECT().Add(gomock.Any(), tc.user, tc.id, tc.listing.SellerID).Return(nil)
				e.repo.EXPECT().Count(gomock.Any(), tc.user).Return(1, nil)
			}
			count, err := e.uc.Add(context.Background(), tc.user, tc.id)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, count)
			}
		})
	}
}

func TestCart_GroupsBySellerNewestFirst(t *testing.T) {
	e := newCartTestEnv(t)
	sold := cartEntry(201, sellerB, 1850000, false)
	sold.Status = domain.AdStatusSold
	e.repo.EXPECT().Entries(gomock.Any(), buyerID).Return([]domain.CartEntry{cartEntry(102, sellerA, 230000, true), sold, cartEntry(101, sellerA, 1500000, true)}, nil)
	cart, err := e.uc.Get(context.Background(), buyerID)
	require.NoError(t, err)
	require.Equal(t, 3, cart.ItemsCount)
	require.Len(t, cart.Groups, 2)
	require.Equal(t, sellerA, cart.Groups[0].Seller.ID)
	require.Equal(t, int32(102), cart.Groups[0].Items[0].ListingID)
	require.Equal(t, int32(101), cart.Groups[0].Items[1].ListingID)
	require.Equal(t, domain.ReasonSold, cart.Groups[1].Items[0].UnavailableReason())
}

func TestCart_ReservedByAnotherBuyer(t *testing.T) {
	e := newCartTestEnv(t)
	item := cartEntry(101, sellerA, 1500000, true)
	item.Reserved = true
	e.repo.EXPECT().Entries(gomock.Any(), strangerID).Return([]domain.CartEntry{item}, nil)
	cart, err := e.uc.Get(context.Background(), strangerID)
	require.NoError(t, err)
	require.Equal(t, domain.ReasonReserved, cart.Groups[0].Items[0].UnavailableReason())
	require.Nil(t, cart.Groups[0].Items[0].DeliveryFrom)
	item.Reserved = false
	e.repo.EXPECT().Entries(gomock.Any(), strangerID).Return([]domain.CartEntry{item}, nil)
	cart, err = e.uc.Get(context.Background(), strangerID)
	require.NoError(t, err)
	require.Empty(t, cart.Groups[0].Items[0].UnavailableReason())
	require.NotNil(t, cart.Groups[0].Items[0].DeliveryFrom)
}

func TestCart_EmptyCartHasNoGroups(t *testing.T) {
	e := newCartTestEnv(t)
	e.repo.EXPECT().Entries(gomock.Any(), strangerID).Return(nil, nil)
	cart, err := e.uc.Get(context.Background(), strangerID)
	require.NoError(t, err)
	require.Zero(t, cart.ItemsCount)
	require.NotNil(t, cart.Groups)
}

func TestCart_BulkAddSkipsUnavailable(t *testing.T) {
	e := newCartTestEnv(t)
	expectTransaction(e.tx)
	gomock.InOrder(e.repo.EXPECT().ListingForCart(gomock.Any(), buyerID, int32(101)).Return(domain.CartListing{SellerID: sellerA, Status: domain.AdStatusActive}, nil), e.repo.EXPECT().Add(gomock.Any(), buyerID, int32(101), sellerA).Return(nil),
		e.repo.EXPECT().ListingForCart(gomock.Any(), buyerID, int32(102)).Return(domain.CartListing{SellerID: sellerA, Status: domain.AdStatusClosed}, nil),
		e.repo.EXPECT().ListingForCart(gomock.Any(), buyerID, int32(201)).Return(domain.CartListing{SellerID: sellerB, Status: domain.AdStatusActive}, nil), e.repo.EXPECT().Add(gomock.Any(), buyerID, int32(201), sellerB).Return(nil),
		e.repo.EXPECT().ListingForCart(gomock.Any(), buyerID, int32(999)).Return(domain.CartListing{}, domain.ErrListingNotFound), e.repo.EXPECT().Count(gomock.Any(), buyerID).Return(2, nil))
	count, skipped, err := e.uc.BulkAdd(context.Background(), buyerID, []int32{101, 102, 201, 999, 101})
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, []int32{102, 999}, skipped)
}

func TestCart_RemoveAndValidateIDs(t *testing.T) {
	e := newCartTestEnv(t)
	gomock.InOrder(
		e.repo.EXPECT().Remove(gomock.Any(), buyerID, []int32{101, 201, 555}).Return(nil),
		e.repo.EXPECT().Count(gomock.Any(), buyerID).Return(1, nil),
	)
	count, err := e.uc.Remove(context.Background(), buyerID, []int32{101, 201, 555})
	require.NoError(t, err)
	require.Equal(t, 1, count)
	_, err = e.uc.Remove(context.Background(), buyerID, nil)
	require.ErrorIs(t, err, domain.ErrInvalidListingIDs)
	_, err = e.uc.Remove(context.Background(), buyerID, []int32{0})
	require.ErrorIs(t, err, domain.ErrInvalidListingIDs)
	_, _, err = e.uc.BulkAdd(context.Background(), buyerID, make([]int32, domain.MaxCartBatch+1))
	require.ErrorIs(t, err, domain.ErrInvalidListingIDs)
}

func TestCart_DeletedListingStaysInCart(t *testing.T) {
	e := newCartTestEnv(t)
	e.repo.EXPECT().Entries(gomock.Any(), buyerID).Return([]domain.CartEntry{cartEntry(102, sellerA, 230000, true), {ListingID: 101, Status: domain.ListingDeleted, Seller: domain.CartSeller{ID: sellerA}}}, nil)
	cart, err := e.uc.Get(context.Background(), buyerID)
	require.NoError(t, err)
	require.Equal(t, 2, cart.ItemsCount)
	items := cart.Groups[0].Items
	require.Equal(t, domain.ReasonDeleted, items[1].UnavailableReason())
	require.Nil(t, items[1].DeliveryFrom)
	require.Equal(t, int64(19000), *items[0].DeliveryFrom)
}

func TestCart_BulkAddLocksInIDOrder(t *testing.T) {
	e := newCartTestEnv(t)
	expectTransaction(e.tx)
	var previous *gomock.Call
	for _, id := range []int32{101, 102, 201, 999} {
		call := e.repo.EXPECT().ListingForCart(gomock.Any(), buyerID, id)
		if previous != nil {
			call.After(previous)
		}
		if id == 999 {
			call.Return(domain.CartListing{}, domain.ErrListingNotFound)
			previous = call
		} else {
			call.Return(domain.CartListing{SellerID: sellerA, Status: domain.AdStatusActive}, nil)
			previous = e.repo.EXPECT().Add(gomock.Any(), buyerID, id, sellerA).Return(nil).After(call)
		}
	}
	e.repo.EXPECT().Count(gomock.Any(), buyerID).Return(3, nil).After(previous)
	_, skipped, err := e.uc.BulkAdd(context.Background(), buyerID, []int32{201, 999, 101, 102})
	require.NoError(t, err)
	require.Equal(t, []int32{999}, skipped)
}
