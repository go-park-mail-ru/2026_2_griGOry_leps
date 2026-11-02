package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type OrderRepository struct {
	db DB
}

func NewOrderRepository(db DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) LockBuyer(ctx context.Context, buyerID int32) error {
	if _, err := conn(ctx, r.db).Exec(ctx, lockBuyerQuery, buyerID); err != nil {
		return fmt.Errorf("lock buyer orders: %w", err)
	}
	return nil
}

func (r *OrderRepository) FindByKey(ctx context.Context, buyerID int32, key string) (int32, bool, error) {
	var id int32
	err := conn(ctx, r.db).QueryRow(ctx, orderIDByKeyQuery, buyerID, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("find order by key: %w", err)
	}
	return id, true, nil
}

func (r *OrderRepository) CancelOwnOverlapping(ctx context.Context, buyerID int32, listingIDs []int32) error {
	if _, err := conn(ctx, r.db).Exec(ctx, cancelOwnOverlappingQuery, buyerID, listingIDs); err != nil {
		return fmt.Errorf("cancel overlapping orders: %w", err)
	}
	return nil
}

func (r *OrderRepository) Create(ctx context.Context, o domain.Order, key string) (int32, error) {
	db := conn(ctx, r.db)

	var id int32
	err := db.QueryRow(ctx, createOrderQuery,
		o.BuyerID, key, o.Status, o.PaymentMethod, o.PaymentStatus, o.PaymentToken,
		o.PayUntil, o.Recipient.FirstName, o.Recipient.Phone, o.AddressID, o.AddressText, o.Date,
		o.TimeSlot, o.Comment, o.ItemsTotal, o.DeliveryTotal,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create order: %w", err)
	}

	sellerIDs := make([]int32, len(o.BySeller))
	methods := make([]string, len(o.BySeller))
	prices := make([]int64, len(o.BySeller))
	for i, s := range o.BySeller {
		sellerIDs[i], methods[i], prices[i] = s.SellerID, s.Method, s.Price
	}
	if _, err := db.Exec(ctx, createOrderSellersQuery, id, sellerIDs, methods, prices); err != nil {
		return 0, fmt.Errorf("create order sellers: %w", err)
	}

	adIDs := make([]*int32, len(o.Items))
	itemSellers := make([]int32, len(o.Items))
	titles := make([]string, len(o.Items))
	itemPrices := make([]int64, len(o.Items))
	images := make([]*string, len(o.Items))
	for i, it := range o.Items {
		adIDs[i], itemSellers[i], titles[i], itemPrices[i], images[i] = it.ListingID, it.SellerID, it.Title, it.Price, it.ImageURL
	}
	if _, err := db.Exec(ctx, createOrderItemsQuery, id, adIDs, itemSellers, titles, itemPrices, images); err != nil {
		return 0, fmt.Errorf("create order items: %w", err)
	}
	return id, nil
}

func (r *OrderRepository) Get(ctx context.Context, id int32) (domain.Order, error) {
	return r.get(ctx, getOrderQuery, id)
}

func (r *OrderRepository) GetForUpdate(ctx context.Context, id int32) (domain.Order, error) {
	return r.get(ctx, getOrderQuery+" FOR UPDATE", id)
}

func (r *OrderRepository) get(ctx context.Context, query string, id int32) (domain.Order, error) {
	db := conn(ctx, r.db)

	var o domain.Order
	err := db.QueryRow(ctx, query, id).Scan(
		&o.ID, &o.BuyerID, &o.Status, &o.PaymentMethod, &o.PaymentStatus, &o.PaymentToken, &o.PaymentConfirmAt,
		&o.PayUntil, &o.Recipient.FirstName, &o.Recipient.Phone, &o.AddressID, &o.AddressText, &o.Date,
		&o.TimeSlot, &o.Comment, &o.ItemsTotal, &o.DeliveryTotal, &o.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("get order: %w", err)
	}

	rows, err := db.Query(ctx, orderSellersQuery, id)
	if err != nil {
		return domain.Order{}, fmt.Errorf("get order sellers: %w", err)
	}
	o.BySeller, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.SellerDelivery, error) {
		var s domain.SellerDelivery
		err := row.Scan(&s.SellerID, &s.Method, &s.Price)
		return s, err
	})
	if err != nil {
		return domain.Order{}, fmt.Errorf("scan order sellers: %w", err)
	}

	rows, err = db.Query(ctx, orderItemsQuery, id)
	if err != nil {
		return domain.Order{}, fmt.Errorf("get order items: %w", err)
	}
	o.Items, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.OrderItem, error) {
		var it domain.OrderItem
		err := row.Scan(&it.ListingID, &it.SellerID, &it.Title, &it.Price, &it.ImageURL)
		return it, err
	})
	if err != nil {
		return domain.Order{}, fmt.Errorf("scan order items: %w", err)
	}
	return o, nil
}

func (r *OrderRepository) IDByToken(ctx context.Context, token string) (int32, error) {
	var id int32
	err := conn(ctx, r.db).QueryRow(ctx, orderIDByTokenQuery, token).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrOrderNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("find order by token: %w", err)
	}
	return id, nil
}

func (r *OrderRepository) UpdatePayment(ctx context.Context, o domain.Order) error {
	_, err := conn(ctx, r.db).Exec(ctx, updateOrderPaymentQuery,
		o.ID, o.Status, o.PaymentMethod, o.PaymentStatus, o.PaymentToken, o.PaymentConfirmAt)
	if err != nil {
		return fmt.Errorf("update order payment: %w", err)
	}
	return nil
}

func (r *OrderRepository) MarkSold(ctx context.Context, orderID int32) error {
	if _, err := conn(ctx, r.db).Exec(ctx, markOrderSoldQuery, orderID); err != nil {
		return fmt.Errorf("mark order listings sold: %w", err)
	}
	return nil
}

func (r *OrderRepository) Due(ctx context.Context, now time.Time) ([]int32, error) {
	rows, err := conn(ctx, r.db).Query(ctx, dueOrdersQuery, now)
	if err != nil {
		return nil, fmt.Errorf("due orders: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return nil, fmt.Errorf("scan due orders: %w", err)
	}
	return ids, nil
}
