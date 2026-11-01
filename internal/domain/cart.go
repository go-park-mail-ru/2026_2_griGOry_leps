package domain

import "errors"

const (
	DeliveryCourier     = "courier"
	DeliveryPickupPoint = "pickup_point"
	DeliverySelfPickup  = "self_pickup"

	PaymentCard = "card"
	PaymentSBP  = "sbp"
	PaymentCOD  = "cod"

	UnavailableSold    = "sold"
	UnavailableClosed  = "closed"
	UnavailableDeleted = "deleted"

	MaxCartBatch = 100

	CourierPrice     = 39000
	PickupPointPrice = 19000
)

var (
	ErrOwnListing         = errors.New("cannot add own listing")
	ErrListingUnavailable = errors.New("listing is unavailable")
	ErrInvalidListingIDs  = errors.New("invalid listing_ids")
)

type ItemsUnavailableError struct {
	ListingIDs []int32
}

func (e *ItemsUnavailableError) Error() string { return "cart items are unavailable" }

type CartSeller struct {
	ID        int32
	FirstName string
	Avatar    string
	Rating    float64
}

type CartEntry struct {
	ListingID   int32
	Title       string
	Price       *int64
	ImageURL    *string
	Address     *string
	HasDelivery bool
	Status      string
	Seller      CartSeller
}

func (e CartEntry) UnavailableReason() string {
	switch e.Status {
	case AdStatusActive:
		return ""
	case AdStatusSold:
		return UnavailableSold
	default:
		return UnavailableClosed
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

type CartListing struct {
	SellerID int32
	Status   string
}
