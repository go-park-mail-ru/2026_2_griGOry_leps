package domain

import "errors"

const (
	ReasonSold    = "sold"
	ReasonClosed  = "closed"
	ReasonDeleted = "deleted"

	ListingDeleted = "deleted"

	MaxCartBatch = 100
)

var (
	ErrOwnListing         = errors.New("cannot add own listing")
	ErrListingUnavailable = errors.New("listing is unavailable")
	ErrInvalidListingIDs  = errors.New("invalid listing_ids")
)

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
	DeliveryFrom *int64
	Seller       CartSeller
}

func (e CartEntry) UnavailableReason() string {
	switch e.Status {
	case AdStatusActive:
		return ""
	case AdStatusSold:
		return ReasonSold
	case ListingDeleted:
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

type CartListing struct {
	SellerID int32
	Status   string
}
