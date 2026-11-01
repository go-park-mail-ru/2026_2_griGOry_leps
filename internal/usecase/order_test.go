package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type orderEnv struct {
	uc     *OrderUsecase
	carts  *cartRepoFake
	orders *orderRepoFake
	now    *time.Time
}

func newOrderEnv(t *testing.T) orderEnv {
	t.Helper()
	carts := newCartRepoFake()
	carts.put(101, sellerA, 1500000, true)
	carts.put(102, sellerA, 230000, true)
	carts.put(201, sellerB, 1850000, false)

	now := time.Date(2026, 10, 26, 23, 30, 0, 0, moscow)
	orders := newOrderRepoFake(carts, &now)

	users := newUserRepoFake()
	users.users[buyerID] = domain.User{ID: buyerID, FirstName: "Тимур", Phone: "+79001234567"}
	addresses := &addressRepoFake{}
	_, err := addresses.Create(context.Background(), domain.Address{UserID: buyerID, Street: "Тверская", House: "12", IsDefault: true})
	require.NoError(t, err)

	uc := NewOrderUsecase(carts, orders, addresses, users, txFake{}, testPrices)
	uc.now = func() time.Time { return now }
	for _, id := range []int32{101, 102, 201} {
		require.NoError(t, carts.Add(context.Background(), buyerID, id, carts.catalog[id].Seller.ID))
	}
	return orderEnv{uc: uc, carts: carts, orders: orders, now: &now}
}

func courierInput(key string) domain.OrderInput {
	addressID, date, slot := int32(1), "2026-10-27", "10:00-14:00"
	return domain.OrderInput{
		ListingIDs: []int32{101, 201},
		BySeller: []domain.SellerDelivery{
			{SellerID: sellerA, Method: domain.DeliveryCourier},
			{SellerID: sellerB, Method: domain.DeliverySelfPickup},
		},
		AddressID:      &addressID,
		Date:           &date,
		TimeSlot:       &slot,
		Recipient:      domain.Recipient{FirstName: "Тимур", Phone: "8 900 123-45-67"},
		PaymentMethod:  domain.PaymentCard,
		IdempotencyKey: key,
	}
}

const (
	key1 = "11111111-1111-4111-8111-111111111111"
	key2 = "22222222-2222-4222-8222-222222222222"
)

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

func TestOrder_CreateValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.OrderInput)
		want   error
	}{
		{"bad key", func(in *domain.OrderInput) { in.IdempotencyKey = "x" }, domain.ErrInvalidIdempotency},
		{"no ids", func(in *domain.OrderInput) { in.ListingIDs = nil }, domain.ErrInvalidListingIDs},
		{"courier without delivery", func(in *domain.OrderInput) { in.BySeller[1].Method = domain.DeliveryCourier }, domain.ErrDeliveryNotAvailable},
		{"missing seller", func(in *domain.OrderInput) { in.BySeller = in.BySeller[:1] }, domain.ErrDeliveryNotAvailable},
		{"unknown method", func(in *domain.OrderInput) { in.BySeller[0].Method = "drone" }, domain.ErrDeliveryNotAvailable},
		{"no address", func(in *domain.OrderInput) { in.AddressID = nil }, domain.ErrDeliveryAddress},
		{"foreign address", func(in *domain.OrderInput) { id := int32(77); in.AddressID = &id }, domain.ErrDeliveryAddress},
		{"blank new address", func(in *domain.OrderInput) { in.AddressID = nil; s := "  "; in.AddressText = &s }, domain.ErrDeliveryAddress},
		{"today is not a slot", func(in *domain.OrderInput) { d := "2026-10-26"; in.Date = &d }, domain.ErrDeliveryDate},
		{"bad slot", func(in *domain.OrderInput) { s := "09:00-10:00"; in.TimeSlot = &s }, domain.ErrDeliveryTimeSlot},
		{"long comment", func(in *domain.OrderInput) { in.Comment = string(make([]rune, 201)) }, domain.ErrOrderCommentTooLong},
		{"no name", func(in *domain.OrderInput) { in.Recipient.FirstName = " " }, domain.ErrMissingFirstName},
		{"bad phone", func(in *domain.OrderInput) { in.Recipient.Phone = "123" }, domain.ErrInvalidPhone},
		{"bad payment", func(in *domain.OrderInput) { in.PaymentMethod = "cash" }, domain.ErrInvalidPaymentMethod},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newOrderEnv(t)
			in := courierInput(key1)
			tt.mutate(&in)
			_, err := e.uc.Create(context.Background(), buyerID, in)
			require.ErrorIs(t, err, tt.want)
			require.Empty(t, e.orders.orders)
		})
	}
}

func TestOrder_PickupOnlyNeedsNoAddress(t *testing.T) {
	e := newOrderEnv(t)
	in := courierInput(key1)
	in.BySeller[0].Method = domain.DeliveryPickupPoint
	in.AddressID, in.Date, in.TimeSlot = nil, nil, nil

	o, err := e.uc.Create(context.Background(), buyerID, in)
	require.NoError(t, err)
	require.Nil(t, o.AddressText)
	require.Equal(t, testPrices.PickupPoint, o.DeliveryTotal)
}

func TestOrder_CashOnDeliveryIsPlacedAtOnce(t *testing.T) {
	e := newOrderEnv(t)
	in := courierInput(key1)
	in.PaymentMethod = domain.PaymentCOD

	o, err := e.uc.Create(context.Background(), buyerID, in)
	require.NoError(t, err)
	require.Equal(t, domain.OrderPlaced, o.Status)
	require.Equal(t, domain.PaymentOnDelivery, o.PaymentStatus)
	require.Nil(t, o.PayUntil)
	require.Equal(t, "+79001234567", o.Recipient.Phone)
	require.Equal(t, "Тверская, 12", *o.AddressText)
	require.Equal(t, testPrices.Courier, o.DeliveryTotal)
	require.Equal(t, domain.AdStatusSold, e.carts.catalog[101].Status)
	require.Equal(t, []int32{102}, e.carts.carts[buyerID])
}

func TestOrder_CardOrderWaitsForPayment(t *testing.T) {
	e := newOrderEnv(t)
	ctx := context.Background()

	o, err := e.uc.Create(ctx, buyerID, courierInput(key1))
	require.NoError(t, err)
	require.Equal(t, domain.OrderAwaitingPayment, o.Status)
	require.Equal(t, domain.PaymentPending, o.PaymentStatus)
	require.Equal(t, e.now.Add(time.Hour), *o.PayUntil)
	require.Len(t, e.carts.carts[buyerID], 3)
	require.True(t, e.orders.reserved(strangerID, 101))
	require.False(t, e.orders.reserved(buyerID, 101))

	again, err := e.uc.Create(ctx, buyerID, courierInput(key1))
	require.NoError(t, err)
	require.Equal(t, o.ID, again.ID)
	require.Len(t, e.orders.orders, 1)

	_, err = e.uc.Get(ctx, strangerID, o.ID)
	require.ErrorIs(t, err, domain.ErrOrderNotFound)
}

func TestOrder_ExpiresAfterAnHour(t *testing.T) {
	e := newOrderEnv(t)
	ctx := context.Background()
	o, err := e.uc.Create(ctx, buyerID, courierInput(key1))
	require.NoError(t, err)

	*e.now = e.now.Add(time.Hour)
	expired, err := e.uc.Get(ctx, buyerID, o.ID)
	require.NoError(t, err)
	require.Equal(t, domain.OrderCancelled, expired.Status)
	require.False(t, e.orders.reserved(strangerID, 101))
	require.Len(t, e.carts.carts[buyerID], 3)
}

func TestOrder_NewCheckoutCancelsOwnUnpaidOrder(t *testing.T) {
	e := newOrderEnv(t)
	ctx := context.Background()
	first, err := e.uc.Create(ctx, buyerID, courierInput(key1))
	require.NoError(t, err)

	in := courierInput(key2)
	in.ListingIDs = []int32{101, 102, 201}
	second, err := e.uc.Create(ctx, buyerID, in)
	require.NoError(t, err)
	require.Equal(t, domain.OrderAwaitingPayment, second.Status)
	require.Equal(t, domain.OrderCancelled, e.orders.orders[first.ID].Status)
}

func TestOrder_SelfPickupShowsEveryListingAddress(t *testing.T) {
	e := newOrderEnv(t)
	other := "Москва, м. Аэропорт"
	c := e.carts.catalog[102]
	c.Address = &other
	e.carts.catalog[102] = c

	q, err := e.uc.Quote(context.Background(), buyerID, []int32{101, 102})
	require.NoError(t, err)
	require.Equal(t, "Москва, м. Аэропорт; Москва, м. Сокол · договоритесь с продавцом", q.Sellers[0].Options[2].Hint)
	require.Equal(t, int64(39000), q.Sellers[0].Options[0].Price)
}

func TestOrder_ReplayReturnsOrderEvenWhenListingsAreSold(t *testing.T) {
	e := newOrderEnv(t)
	ctx := context.Background()
	in := courierInput(key1)
	in.PaymentMethod = domain.PaymentCOD

	first, err := e.uc.Create(ctx, buyerID, in)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusSold, e.carts.catalog[101].Status)

	again, err := e.uc.Create(ctx, buyerID, in)
	require.NoError(t, err)
	require.Equal(t, first.ID, again.ID)
	require.Len(t, e.orders.orders, 1)
}
