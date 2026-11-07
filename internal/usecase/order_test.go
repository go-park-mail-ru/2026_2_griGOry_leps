package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type orderTestEnv struct {
	uc        *OrderUsecase
	carts     *MockCartRepository
	orders    *MockOrderRepository
	addresses *MockAddressRepository
	users     *MockUserRepository
	tx        *MockTransactor
	now       time.Time
}

func newOrderTestEnv(t *testing.T) *orderTestEnv {
	t.Helper()
	c := gomock.NewController(t)
	e := &orderTestEnv{carts: NewMockCartRepository(c), orders: NewMockOrderRepository(c), addresses: NewMockAddressRepository(c), users: NewMockUserRepository(c), tx: NewMockTransactor(c), now: time.Date(2026, 10, 26, 23, 30, 0, 0, moscow)}
	e.uc = NewOrderUsecase(e.carts, e.orders, e.addresses, e.users, e.tx, testPrices)
	e.uc.now = func() time.Time { return e.now }
	return e
}

func orderEntries() []domain.CartEntry {
	return []domain.CartEntry{cartEntry(201, sellerB, 1850000, false), cartEntry(101, sellerA, 1500000, true)}
}

func orderAddress() domain.Address {
	return domain.Address{ID: 1, UserID: buyerID, Street: "Тверская", House: "12", IsDefault: true}
}

func expectOrderStart(e *orderTestEnv, userID int32, in domain.OrderInput, entries []domain.CartEntry) {
	expectTransaction(e.tx)
	gomock.InOrder(
		e.orders.EXPECT().LockBuyer(gomock.Any(), userID).Return(nil),
		e.orders.EXPECT().FindByKey(gomock.Any(), userID, in.IdempotencyKey).Return(int32(0), false, nil),
		e.orders.EXPECT().CancelOwnOverlapping(gomock.Any(), userID, in.ListingIDs).Return(nil),
		e.carts.EXPECT().LockListings(gomock.Any(), in.ListingIDs).Return(nil),
		e.carts.EXPECT().EntriesByIDs(gomock.Any(), userID, in.ListingIDs).Return(entries, nil),
	)
}

func expectedCourierOrder(e *orderTestEnv) domain.Order {
	return domain.Order{BuyerID: buyerID, Status: domain.OrderAwaitingPayment, PaymentMethod: domain.PaymentCard, PaymentStatus: domain.PaymentPending, PayUntil: ptr(e.now.Add(time.Hour)), Items: []domain.OrderItem{{ListingID: ptr(int32(201)), SellerID: sellerB, Title: "item", Price: 1850000}, {ListingID: ptr(int32(101)), SellerID: sellerA, Title: "item", Price: 1500000}}, ItemsTotal: 3350000, BySeller: []domain.SellerDelivery{{SellerID: sellerB, Method: domain.DeliverySelfPickup}, {SellerID: sellerA, Method: domain.DeliveryCourier, Price: 39000}}, DeliveryTotal: 39000, AddressID: ptr(int32(1)), AddressText: ptr("Тверская, 12"), Date: ptr("2026-10-27"), TimeSlot: ptr("10:00-14:00"), Recipient: domain.Recipient{FirstName: "Тимур", Phone: "+79001234567"}}
}

func expectOrderCreate(t *testing.T, e *orderTestEnv, in domain.OrderInput, want domain.Order) {
	t.Helper()
	var created domain.Order
	e.orders.EXPECT().Create(gomock.Any(), gomock.Any(), in.IdempotencyKey).DoAndReturn(func(_ context.Context, got domain.Order, _ string) (int32, error) {
		if want.PaymentMethod != domain.PaymentCOD {
			require.NotNil(t, got.PaymentToken)
			require.Len(t, *got.PaymentToken, 64)
			want.PaymentToken = got.PaymentToken
		}
		require.Equal(t, want, got)
		created = got
		created.ID = 7
		return int32(7), nil
	})
	expectTransaction(e.tx)
	e.orders.EXPECT().GetForUpdate(gomock.Any(), int32(7)).DoAndReturn(func(context.Context, int32) (domain.Order, error) { return created, nil })
}

func pendingOrder(e *orderTestEnv) domain.Order {
	o := expectedCourierOrder(e)
	o.ID = 7
	o.PaymentToken = ptr("token")
	return o
}

func expectOrderRead(e *orderTestEnv, o domain.Order) {
	expectTransaction(e.tx)
	e.orders.EXPECT().GetForUpdate(gomock.Any(), o.ID).Return(o, nil)
}

func expectPlace(e *orderTestEnv, o domain.Order) {
	gomock.InOrder(
		e.orders.EXPECT().MarkSold(gomock.Any(), o.ID).Return(nil),
		e.carts.EXPECT().Remove(gomock.Any(), o.BuyerID, []int32{201, 101}).Return(nil),
	)
}

func expectReplay(e *orderTestEnv, key string, o domain.Order) {
	expectTransaction(e.tx)
	gomock.InOrder(
		e.orders.EXPECT().LockBuyer(gomock.Any(), buyerID).Return(nil),
		e.orders.EXPECT().FindByKey(gomock.Any(), buyerID, key).Return(o.ID, true, nil),
	)
	expectOrderRead(e, o)
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
	e := newOrderTestEnv(t)
	entries := append(orderEntries(), cartEntry(102, sellerA, 230000, true))
	e.carts.EXPECT().EntriesByIDs(gomock.Any(), buyerID, []int32{101, 102, 201}).Return(entries, nil)
	e.users.EXPECT().GetByID(gomock.Any(), buyerID).Return(domain.User{FirstName: "Тимур", Phone: "+79001234567"}, nil)
	e.addresses.EXPECT().List(gomock.Any(), buyerID).Return([]domain.Address{orderAddress()}, nil)
	q, err := e.uc.Quote(context.Background(), buyerID, []int32{101, 102, 201})
	require.NoError(t, err)
	require.Equal(t, int64(3580000), q.ItemsTotal)
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
	e := newOrderTestEnv(t)
	sold := cartEntry(102, sellerA, 230000, true)
	sold.Status = domain.AdStatusSold
	e.carts.EXPECT().EntriesByIDs(gomock.Any(), buyerID, []int32{101, 102, 999}).Return([]domain.CartEntry{cartEntry(101, sellerA, 1500000, true), sold}, nil)
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
			e := newOrderTestEnv(t)
			in := courierInput(key1)
			tt.mutate(&in)
			if tt.name != "bad key" && tt.name != "no ids" {
				expectOrderStart(e, buyerID, in, orderEntries())
				switch tt.name {
				case "courier without delivery", "missing seller", "unknown method", "no address", "blank new address":
				case "foreign address":
					e.addresses.EXPECT().GetForUpdate(gomock.Any(), buyerID, int32(77)).Return(domain.Address{}, domain.ErrAddressNotFound)
				default:
					e.addresses.EXPECT().GetForUpdate(gomock.Any(), buyerID, int32(1)).Return(orderAddress(), nil)
				}
			}
			_, err := e.uc.Create(context.Background(), buyerID, in)
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestOrder_PickupOnlyNeedsNoAddress(t *testing.T) {
	e := newOrderTestEnv(t)
	in := courierInput(key1)
	in.BySeller[0].Method = domain.DeliveryPickupPoint
	in.AddressID = nil
	in.Date = nil
	in.TimeSlot = nil
	expectOrderStart(e, buyerID, in, orderEntries())
	want := expectedCourierOrder(e)
	want.AddressID = nil
	want.AddressText = nil
	want.Date = nil
	want.TimeSlot = nil
	want.BySeller[1].Method = domain.DeliveryPickupPoint
	want.BySeller[1].Price = 19000
	want.DeliveryTotal = 19000
	expectOrderCreate(t, e, in, want)
	o, err := e.uc.Create(context.Background(), buyerID, in)
	require.NoError(t, err)
	require.Nil(t, o.AddressText)
	require.Equal(t, testPrices.PickupPoint, o.DeliveryTotal)
}

func TestOrder_CashOnDeliveryIsPlacedAtOnce(t *testing.T) {
	e := newOrderTestEnv(t)
	in := courierInput(key1)
	in.PaymentMethod = domain.PaymentCOD
	expectOrderStart(e, buyerID, in, orderEntries())
	e.addresses.EXPECT().GetForUpdate(gomock.Any(), buyerID, int32(1)).Return(orderAddress(), nil)
	want := expectedCourierOrder(e)
	want.PaymentMethod = domain.PaymentCOD
	want.Status = domain.OrderPlaced
	want.PaymentStatus = domain.PaymentOnDelivery
	want.PayUntil = nil
	expectOrderCreate(t, e, in, want)
	placed := want
	placed.ID = 7
	expectPlace(e, placed)
	o, err := e.uc.Create(context.Background(), buyerID, in)
	require.NoError(t, err)
	want.ID = 7
	require.Equal(t, want, o)
}

func TestOrder_CardPaymentFlow(t *testing.T) {
	e := newOrderTestEnv(t)
	in := courierInput(key1)
	expectOrderStart(e, buyerID, in, orderEntries())
	e.addresses.EXPECT().GetForUpdate(gomock.Any(), buyerID, int32(1)).Return(orderAddress(), nil)
	expectOrderCreate(t, e, in, expectedCourierOrder(e))
	o, err := e.uc.Create(context.Background(), buyerID, in)
	require.NoError(t, err)
	require.Equal(t, domain.OrderAwaitingPayment, o.Status)
	require.Equal(t, domain.PaymentPending, o.PaymentStatus)
	require.Equal(t, e.now.Add(time.Hour), *o.PayUntil)
	expectReplay(e, key1, o)
	again, err := e.uc.Create(context.Background(), buyerID, in)
	require.NoError(t, err)
	require.Equal(t, o.ID, again.ID)
	expectOrderRead(e, o)
	_, err = e.uc.Get(context.Background(), strangerID, o.ID)
	require.ErrorIs(t, err, domain.ErrOrderNotFound)
	expectTransaction(e.tx)
	gomock.InOrder(
		e.orders.EXPECT().IDByToken(gomock.Any(), *o.PaymentToken).Return(o.ID, nil),
		e.orders.EXPECT().GetForUpdate(gomock.Any(), o.ID).Return(o, nil),
		e.carts.EXPECT().LockListings(gomock.Any(), []int32{201, 101}).Return(nil),
	)
	failed := o
	failed.PaymentStatus = domain.PaymentFailed
	e.orders.EXPECT().UpdatePayment(gomock.Any(), failed).Return(nil)
	_, err = e.uc.TestPayment(context.Background(), *o.PaymentToken, false)
	require.NoError(t, err)
	expectOrderRead(e, failed)
	got, err := e.uc.Get(context.Background(), buyerID, o.ID)
	require.NoError(t, err)
	require.Equal(t, domain.PaymentFailed, got.PaymentStatus)
	expectOrderRead(e, failed)
	e.carts.EXPECT().LockListings(gomock.Any(), []int32{201, 101}).Return(nil)
	retried := failed
	retried.PaymentMethod = domain.PaymentSBP
	retried.PaymentStatus = domain.PaymentPending
	e.orders.EXPECT().UpdatePayment(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, got domain.Order) error {
		require.NotNil(t, got.PaymentToken)
		require.NotEqual(t, *o.PaymentToken, *got.PaymentToken)
		retried.PaymentToken = got.PaymentToken
		require.Equal(t, retried, got)
		return nil
	})
	e.orders.EXPECT().Get(gomock.Any(), o.ID).DoAndReturn(func(context.Context, int32) (domain.Order, error) { return retried, nil })
	got, err = e.uc.Pay(context.Background(), buyerID, o.ID, domain.PaymentSBP)
	require.NoError(t, err)
	require.Equal(t, retried, got)
	expectTransaction(e.tx)
	e.orders.EXPECT().IDByToken(gomock.Any(), *o.PaymentToken).Return(int32(0), domain.ErrOrderNotFound)
	_, err = e.uc.TestPayment(context.Background(), *o.PaymentToken, true)
	require.ErrorIs(t, err, domain.ErrOrderNotFound)
	expectTransaction(e.tx)
	e.orders.EXPECT().IDByToken(gomock.Any(), *retried.PaymentToken).Return(o.ID, nil)
	e.orders.EXPECT().GetForUpdate(gomock.Any(), o.ID).Return(retried, nil)
	e.carts.EXPECT().LockListings(gomock.Any(), []int32{201, 101}).Return(nil)
	processing := retried
	processing.PaymentStatus = domain.PaymentProcessing
	processing.PaymentConfirmAt = ptr(e.now.Add(bankConfirmDelay))
	e.orders.EXPECT().UpdatePayment(gomock.Any(), processing).Return(nil)
	_, err = e.uc.TestPayment(context.Background(), *retried.PaymentToken, true)
	require.NoError(t, err)
	expectOrderRead(e, processing)
	e.carts.EXPECT().LockListings(gomock.Any(), []int32{201, 101}).Return(nil)
	_, err = e.uc.Pay(context.Background(), buyerID, o.ID, domain.PaymentCard)
	require.ErrorIs(t, err, domain.ErrOrderNotPayable)
	e.now = e.now.Add(bankConfirmDelay)
	expectOrderRead(e, processing)
	placed := processing
	placed.Status = domain.OrderPlaced
	placed.PaymentStatus = domain.PaymentPaid
	placed.PaymentToken = nil
	placed.PaymentConfirmAt = nil
	e.orders.EXPECT().UpdatePayment(gomock.Any(), placed).Return(nil)
	expectPlace(e, placed)
	got, err = e.uc.Get(context.Background(), buyerID, o.ID)
	require.NoError(t, err)
	require.Equal(t, placed, got)
	expectOrderRead(e, placed)
	e.carts.EXPECT().LockListings(gomock.Any(), []int32{201, 101}).Return(nil)
	_, err = e.uc.Pay(context.Background(), buyerID, o.ID, domain.PaymentCard)
	require.ErrorIs(t, err, domain.ErrOrderNotPayable)
}

func TestOrder_ExpiresAfterAnHour(t *testing.T) {
	e := newOrderTestEnv(t)
	o := pendingOrder(e)
	e.now = e.now.Add(time.Hour)
	e.orders.EXPECT().Due(gomock.Any(), e.now).Return([]int32{o.ID}, nil)
	expectOrderRead(e, o)
	cancelled := o
	cancelled.Status = domain.OrderCancelled
	cancelled.PaymentToken = nil
	e.orders.EXPECT().UpdatePayment(gomock.Any(), cancelled).Return(nil)
	require.NoError(t, e.uc.RunMaintenance(context.Background()))
	expectOrderRead(e, cancelled)
	got, err := e.uc.Get(context.Background(), buyerID, o.ID)
	require.NoError(t, err)
	require.Equal(t, domain.OrderCancelled, got.Status)
}

func TestOrder_MaintenanceSkipsBrokenOrder(t *testing.T) {
	e := newOrderTestEnv(t)
	o := pendingOrder(e)
	o.ID = 8
	e.now = e.now.Add(time.Hour)
	e.orders.EXPECT().Due(gomock.Any(), e.now).Return([]int32{7, 8}, nil)
	expectTransaction(e.tx)
	broken := errors.New("broken order")
	e.orders.EXPECT().GetForUpdate(gomock.Any(), int32(7)).Return(domain.Order{}, broken)
	expectOrderRead(e, o)
	cancelled := o
	cancelled.Status = domain.OrderCancelled
	cancelled.PaymentToken = nil
	e.orders.EXPECT().UpdatePayment(gomock.Any(), cancelled).Return(nil)
	require.ErrorIs(t, e.uc.RunMaintenance(context.Background()), broken)
}

func TestOrder_NewCheckoutCancelsOwnUnpaidOrder(t *testing.T) {
	e := newOrderTestEnv(t)
	in := courierInput(key2)
	expectOrderStart(e, buyerID, in, orderEntries())
	e.addresses.EXPECT().GetForUpdate(gomock.Any(), buyerID, int32(1)).Return(orderAddress(), nil)
	expectOrderCreate(t, e, in, expectedCourierOrder(e))
	o, err := e.uc.Create(context.Background(), buyerID, in)
	require.NoError(t, err)
	require.Equal(t, domain.OrderAwaitingPayment, o.Status)
}

func TestOrder_SelfPickupShowsEveryListingAddress(t *testing.T) {
	e := newOrderTestEnv(t)
	a := cartEntry(102, sellerA, 230000, true)
	a.Address = ptr("Москва, м. Аэропорт")
	b := cartEntry(101, sellerA, 1500000, true)
	b.Address = ptr("Москва, м. Сокол")
	e.carts.EXPECT().EntriesByIDs(gomock.Any(), buyerID, []int32{101, 102}).Return([]domain.CartEntry{a, b}, nil)
	e.users.EXPECT().GetByID(gomock.Any(), buyerID).Return(domain.User{}, nil)
	e.addresses.EXPECT().List(gomock.Any(), buyerID).Return(nil, nil)
	q, err := e.uc.Quote(context.Background(), buyerID, []int32{101, 102})
	require.NoError(t, err)
	require.Equal(t, *a.Address+"; "+*b.Address+" · "+selfPickupHintEnd, q.Sellers[0].Options[2].Hint)
	require.Equal(t, int64(39000), q.Sellers[0].Options[0].Price)
}

func TestOrder_ReplayReturnsOrderEvenWhenListingsAreSold(t *testing.T) {
	e := newOrderTestEnv(t)
	o := pendingOrder(e)
	o.Status = domain.OrderPlaced
	o.PaymentStatus = domain.PaymentOnDelivery
	o.PaymentMethod = domain.PaymentCOD
	o.PaymentToken = nil
	o.PayUntil = nil
	expectReplay(e, key1, o)
	got, err := e.uc.Create(context.Background(), buyerID, courierInput(key1))
	require.NoError(t, err)
	require.Equal(t, o, got)
}

func TestOrder_ProcessingPaymentKeepsReservationAfterDeadline(t *testing.T) {
	e := newOrderTestEnv(t)
	o := pendingOrder(e)
	o.PaymentStatus = domain.PaymentProcessing
	o.PaymentConfirmAt = ptr(e.now.Add(time.Hour + bankConfirmDelay))
	e.now = e.now.Add(time.Hour + time.Second)
	expectOrderRead(e, o)
	got, err := e.uc.Get(context.Background(), buyerID, o.ID)
	require.NoError(t, err)
	require.Equal(t, domain.PaymentProcessing, got.PaymentStatus)
	in := courierInput(key2)
	in.ListingIDs = []int32{101}
	in.BySeller = in.BySeller[:1]
	reserved := cartEntry(101, sellerA, 1500000, true)
	reserved.Reserved = true
	expectOrderStart(e, buyerID, in, []domain.CartEntry{reserved})
	_, err = e.uc.Create(context.Background(), buyerID, in)
	var unavailable *domain.ItemsUnavailableError
	require.ErrorAs(t, err, &unavailable)
	require.Equal(t, []int32{101}, unavailable.ListingIDs)
	e.now = e.now.Add(bankConfirmDelay)
	expectOrderRead(e, o)
	placed := o
	placed.Status = domain.OrderPlaced
	placed.PaymentStatus = domain.PaymentPaid
	placed.PaymentToken = nil
	placed.PaymentConfirmAt = nil
	e.orders.EXPECT().UpdatePayment(gomock.Any(), placed).Return(nil)
	expectPlace(e, placed)
	got, err = e.uc.Get(context.Background(), buyerID, o.ID)
	require.NoError(t, err)
	require.Equal(t, domain.OrderPlaced, got.Status)
}

func TestOrder_ReplayAfterDeadlineReturnsCancelled(t *testing.T) {
	e := newOrderTestEnv(t)
	o := pendingOrder(e)
	e.now = e.now.Add(time.Hour)
	expectReplay(e, key1, o)
	cancelled := o
	cancelled.Status = domain.OrderCancelled
	cancelled.PaymentToken = nil
	e.orders.EXPECT().UpdatePayment(gomock.Any(), cancelled).Return(nil)
	got, err := e.uc.Create(context.Background(), buyerID, courierInput(key1))
	require.NoError(t, err)
	require.Equal(t, o.ID, got.ID)
	require.Equal(t, domain.OrderCancelled, got.Status)
}

func TestOrder_BankConfirmationLocksListingsAndRechecksDeadline(t *testing.T) {
	e := newOrderTestEnv(t)
	o := pendingOrder(e)
	e.now = e.now.Add(time.Hour)
	expectTransaction(e.tx)
	cancelled := o
	cancelled.Status = domain.OrderCancelled
	cancelled.PaymentToken = nil
	gomock.InOrder(
		e.orders.EXPECT().IDByToken(gomock.Any(), "token").Return(o.ID, nil),
		e.orders.EXPECT().GetForUpdate(gomock.Any(), o.ID).Return(o, nil),
		e.carts.EXPECT().LockListings(gomock.Any(), []int32{201, 101}).Return(nil),
		e.orders.EXPECT().UpdatePayment(gomock.Any(), cancelled).Return(nil),
	)
	id, err := e.uc.TestPayment(context.Background(), "token", true)
	require.NoError(t, err)
	require.Equal(t, o.ID, id)
}

func TestOrder_PayLocksListingsAndRechecksDeadline(t *testing.T) {
	e := newOrderTestEnv(t)
	o := pendingOrder(e)
	e.now = e.now.Add(time.Hour)
	expectTransaction(e.tx)
	cancelled := o
	cancelled.Status = domain.OrderCancelled
	cancelled.PaymentToken = nil
	gomock.InOrder(
		e.orders.EXPECT().GetForUpdate(gomock.Any(), o.ID).Return(o, nil),
		e.carts.EXPECT().LockListings(gomock.Any(), []int32{201, 101}).Return(nil),
		e.orders.EXPECT().UpdatePayment(gomock.Any(), cancelled).Return(nil),
	)
	_, err := e.uc.Pay(context.Background(), buyerID, o.ID, domain.PaymentCOD)
	require.ErrorIs(t, err, domain.ErrOrderNotPayable)
}

func TestAddressText_FitsOrderColumn(t *testing.T) {
	longest := addressText(domain.Address{
		City:      strings.Repeat("я", maxCityLength),
		Street:    strings.Repeat("я", maxStreetLength),
		House:     strings.Repeat("я", maxHouseLength),
		Apartment: strings.Repeat("я", maxApartmentLength),
		Comment:   strings.Repeat("я", maxAddressCommentLength),
	})
	require.LessOrEqual(t, utf8.RuneCountInString(longest), 600)
}

func TestAddressText(t *testing.T) {
	require.Equal(t, "Тверская, 12", addressText(domain.Address{Street: "Тверская", House: "12"}))
	require.Equal(t, "Москва, Тверская, 12, кв. 34, домофон 34", addressText(domain.Address{
		City: "Москва", Street: "Тверская", House: "12", Apartment: "34", Comment: "домофон 34",
	}))
}
