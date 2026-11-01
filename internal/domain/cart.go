package domain

import (
	"errors"
	"time"
)

const (
	DeliveryCourier     = "courier"
	DeliveryPickupPoint = "pickup_point"
	DeliverySelfPickup  = "self_pickup"

	PaymentCard = "card"
	PaymentSBP  = "sbp"
	PaymentCOD  = "cod"

	OrderAwaitingPayment = "awaiting_payment"
	OrderPlaced          = "placed"
	OrderCancelled       = "cancelled"

	PaymentPending    = "pending"
	PaymentProcessing = "processing"
	PaymentPaid       = "paid"
	PaymentFailed     = "failed"
	PaymentOnDelivery = "on_delivery"

	ReasonSold    = "sold"
	ReasonClosed  = "closed"
	ReasonDeleted = "deleted"

	ListingDeleted = "deleted"

	MaxCartBatch = 100
)

var (
	ErrOwnListing           = errors.New("cannot add own listing")
	ErrListingUnavailable   = errors.New("listing is unavailable")
	ErrInvalidListingIDs    = errors.New("invalid listing_ids")
	ErrDeliveryNotAvailable = errors.New("delivery method is not available")
	ErrDeliveryAddress      = errors.New("delivery address is required")
	ErrDeliveryDate         = errors.New("delivery date is required")
	ErrDeliveryTimeSlot     = errors.New("delivery time slot is required")
	ErrOrderCommentTooLong  = errors.New("comment must be at most 200 characters")
	ErrInvalidPaymentMethod = errors.New("invalid payment method")
	ErrInvalidIdempotency   = errors.New("invalid Idempotency-Key header")
	ErrOrderNotFound        = errors.New("order not found")
	ErrOrderNotPayable      = errors.New("order is not payable")
	ErrListingHasOrder      = errors.New("listing has active order")
)

type ItemsUnavailableError struct {
	ListingIDs []int32
}

func (e *ItemsUnavailableError) Error() string { return "cart items are unavailable" }

type ListingOrderError struct {
	OrderID     int32
	OrderNumber int32
}

func (e *ListingOrderError) Error() string { return ErrListingHasOrder.Error() }

func (e *ListingOrderError) Unwrap() error { return ErrListingHasOrder }

type DeliveryPrices struct {
	Courier     int64
	PickupPoint int64
}

func (p DeliveryPrices) Cheapest() int64 {
	return min(p.Courier, p.PickupPoint)
}

type CartSeller struct {
	ID        int32
	FirstName string
	Avatar    string
	Rating    float64
}

type CartEntry struct {
	ListingID    int32
	Title        string
	Price        *int64
	ImageURL     *string
	Address      *string
	HasDelivery  bool
	Status       string
	Reserved     bool
	DeliveryFrom *int64
	Seller       CartSeller
}

func (e CartEntry) UnavailableReason() string {
	switch {
	case e.Status == AdStatusActive && !e.Reserved:
		return ""
	case e.Status == AdStatusActive || e.Status == AdStatusSold:
		return ReasonSold
	case e.Status == ListingDeleted:
		return ReasonDeleted
	default:
		return ReasonClosed
	}
}

type CartGroup struct {
	Seller CartSeller
	Items  []CartEntry
}

type Cart struct {
	ItemsCount int
	Groups     []CartGroup
}

type DeliveryOption struct {
	Method    string
	Price     int64
	Available bool
	Hint      string
}

type QuoteSeller struct {
	SellerID   int32
	FirstName  string
	ItemsCount int
	Options    []DeliveryOption
}

type QuoteAddress struct {
	ID        int32
	Label     string
	Text      string
	IsDefault bool
}

type OrderItem struct {
	ListingID *int32
	SellerID  int32
	Title     string
	Price     int64
	ImageURL  *string
}

type CheckoutQuote struct {
	Items      []OrderItem
	ItemsTotal int64
	Sellers    []QuoteSeller
	Addresses  []QuoteAddress
	Dates      []string
	Times      []string
	Recipient  Recipient
	Payments   []string
}

type Recipient struct {
	FirstName string
	Phone     string
}

type SellerDelivery struct {
	SellerID int32
	Method   string
	Price    int64
}

type OrderInput struct {
	ListingIDs     []int32
	BySeller       []SellerDelivery
	AddressID      *int32
	AddressText    *string
	Date           *string
	TimeSlot       *string
	Comment        string
	Recipient      Recipient
	PaymentMethod  string
	IdempotencyKey string
}

type Order struct {
	ID               int32
	BuyerID          int32
	Status           string
	PaymentMethod    string
	PaymentStatus    string
	PaymentToken     *string
	PaymentConfirmAt *time.Time
	PayUntil         *time.Time
	Recipient        Recipient
	AddressID        *int32
	AddressText      *string
	Date             *string
	TimeSlot         *string
	Comment          string
	ItemsTotal       int64
	DeliveryTotal    int64
	BySeller         []SellerDelivery
	Items            []OrderItem
	CreatedAt        time.Time
}

func (o Order) Number() int32 { return 10000 + o.ID }

type CartListing struct {
	SellerID int32
	Status   string
	Reserved bool
}
