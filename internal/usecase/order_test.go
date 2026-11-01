package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type orderEnv struct {
	uc    *OrderUsecase
	carts *cartRepoFake
}

func newOrderEnv(t *testing.T) orderEnv {
	t.Helper()
	carts := newCartRepoFake()
	carts.put(101, sellerA, 1500000, true)
	carts.put(102, sellerA, 230000, true)
	carts.put(201, sellerB, 1850000, false)

	now := time.Date(2026, 10, 26, 23, 30, 0, 0, moscow)

	users := newUserRepoFake()
	users.users[buyerID] = domain.User{ID: buyerID, FirstName: "Тимур", Phone: "+79001234567"}
	addresses := &addressRepoFake{}
	_, err := addresses.Create(context.Background(), domain.Address{UserID: buyerID, Street: "Тверская", House: "12", IsDefault: true})
	require.NoError(t, err)

	uc := NewOrderUsecase(carts, addresses, users)
	uc.now = func() time.Time { return now }
	for _, id := range []int32{101, 102, 201} {
		require.NoError(t, carts.Add(context.Background(), buyerID, id))
	}
	return orderEnv{uc: uc, carts: carts}
}

func TestOrder_QuoteBuildsSellersAndSlots(t *testing.T) {
	e := newOrderEnv(t)
	q, err := e.uc.Quote(context.Background(), buyerID, []int32{101, 102, 201})
	require.NoError(t, err)

	require.Equal(t, int64(1500000+230000+1850000), q.ItemsTotal)
	require.Len(t, q.Sellers, 2)
	require.Equal(t, sellerB, q.Sellers[0].SellerID)
	require.False(t, q.Sellers[0].Options[0].Available)
	require.True(t, q.Sellers[0].Options[2].Available)
	require.Equal(t, 2, q.Sellers[1].ItemsCount)
	require.True(t, q.Sellers[1].Options[0].Available)
	require.Equal(t, []string{"2026-10-27", "2026-10-28", "2026-10-29", "2026-10-30"}, q.Dates)
	require.Equal(t, "Тверская, 12", q.Addresses[0].Text)
	require.Equal(t, domain.Recipient{FirstName: "Тимур", Phone: "+79001234567"}, q.Recipient)
}

func TestOrder_QuoteUnavailable(t *testing.T) {
	e := newOrderEnv(t)
	c := e.carts.catalog[102]
	c.Status = domain.AdStatusSold
	e.carts.catalog[102] = c

	_, err := e.uc.Quote(context.Background(), buyerID, []int32{101, 102, 999})
	var unavailable *domain.ItemsUnavailableError
	require.ErrorAs(t, err, &unavailable)
	require.Equal(t, []int32{102, 999}, unavailable.ListingIDs)
}
