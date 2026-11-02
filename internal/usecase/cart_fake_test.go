package usecase

import (
	"context"
	"slices"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type cartRepoFake struct {
	catalog map[int32]domain.CartEntry
	carts   map[int32][]int32
	orders  *orderRepoFake
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

func (r *cartRepoFake) entry(userID, id int32) domain.CartEntry {
	e := r.catalog[id]
	if r.orders != nil {
		e.Reserved = r.orders.reserved(userID, id)
	}
	return e
}

func (r *cartRepoFake) Entries(_ context.Context, userID int32) ([]domain.CartEntry, error) {
	ids := r.carts[userID]
	entries := make([]domain.CartEntry, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		entries = append(entries, r.entry(userID, ids[i]))
	}
	return entries, nil
}

func (r *cartRepoFake) EntriesByIDs(ctx context.Context, userID int32, ids []int32) ([]domain.CartEntry, error) {
	all, _ := r.Entries(ctx, userID)
	var entries []domain.CartEntry
	for _, e := range all {
		if slices.Contains(ids, e.ListingID) {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (r *cartRepoFake) LockListings(context.Context, []int32) error { return nil }

func (r *cartRepoFake) ListingForCart(_ context.Context, userID, listingID int32) (domain.CartListing, error) {
	if _, ok := r.catalog[listingID]; !ok {
		return domain.CartListing{}, domain.ErrListingNotFound
	}
	e := r.entry(userID, listingID)
	return domain.CartListing{SellerID: e.Seller.ID, Status: e.Status, Reserved: e.Reserved}, nil
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

type orderRepoFake struct {
	carts  *cartRepoFake
	orders map[int32]domain.Order
	keys   map[string]int32
	now    *time.Time
}

func newOrderRepoFake(carts *cartRepoFake, now *time.Time) *orderRepoFake {
	r := &orderRepoFake{carts: carts, orders: make(map[int32]domain.Order), keys: make(map[string]int32), now: now}
	carts.orders = r
	return r
}

func (r *orderRepoFake) reserved(viewerID, listingID int32) bool {
	for _, o := range r.orders {
		if o.Status != domain.OrderAwaitingPayment || o.PayUntil == nil || !o.PayUntil.After(*r.now) {
			continue
		}
		if o.BuyerID == viewerID && o.PaymentStatus != domain.PaymentProcessing {
			continue
		}
		for _, it := range o.Items {
			if it.ListingID != nil && *it.ListingID == listingID {
				return true
			}
		}
	}
	return false
}

func (r *orderRepoFake) LockBuyer(context.Context, int32) error { return nil }

func (r *orderRepoFake) FindByKey(_ context.Context, buyerID int32, key string) (int32, bool, error) {
	id, ok := r.keys[key]
	if ok && r.orders[id].BuyerID != buyerID {
		return 0, false, nil
	}
	return id, ok, nil
}

func (r *orderRepoFake) CancelOwnOverlapping(_ context.Context, buyerID int32, ids []int32) error {
	for id, o := range r.orders {
		if o.BuyerID != buyerID || o.Status != domain.OrderAwaitingPayment || o.PaymentStatus == domain.PaymentProcessing {
			continue
		}
		if slices.ContainsFunc(o.Items, func(it domain.OrderItem) bool { return it.ListingID != nil && slices.Contains(ids, *it.ListingID) }) {
			o.Status = domain.OrderCancelled
			r.orders[id] = o
		}
	}
	return nil
}

func (r *orderRepoFake) Create(_ context.Context, o domain.Order, key string) (int32, error) {
	o.ID = int32(len(r.orders) + 1)
	o.CreatedAt = *r.now
	r.orders[o.ID] = o
	r.keys[key] = o.ID
	return o.ID, nil
}

func (r *orderRepoFake) Get(_ context.Context, id int32) (domain.Order, error) {
	o, ok := r.orders[id]
	if !ok {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	return o, nil
}

func (r *orderRepoFake) GetForUpdate(ctx context.Context, id int32) (domain.Order, error) {
	return r.Get(ctx, id)
}

func (r *orderRepoFake) UpdatePayment(_ context.Context, o domain.Order) error {
	stored := r.orders[o.ID]
	stored.Status, stored.PaymentMethod, stored.PaymentStatus = o.Status, o.PaymentMethod, o.PaymentStatus
	stored.PaymentToken, stored.PaymentConfirmAt = o.PaymentToken, o.PaymentConfirmAt
	r.orders[o.ID] = stored
	return nil
}

func (r *orderRepoFake) MarkSold(_ context.Context, orderID int32) error {
	for _, it := range r.orders[orderID].Items {
		e := r.carts.catalog[*it.ListingID]
		e.Status = domain.AdStatusSold
		r.carts.catalog[*it.ListingID] = e
	}
	return nil
}
