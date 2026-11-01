package domain

import "errors"

const (
	UnavailableSold    = "sold"
	UnavailableClosed  = "closed"
	UnavailableDeleted = "deleted"

	MaxCartBatch = 100

	PickupPointPrice = 19000
)

var (
	ErrOwnListing         = errors.New("cannot add own listing")
	ErrListingUnavailable = errors.New("listing is unavailable")
	ErrInvalidListingIDs  = errors.New("invalid listing_ids")
)

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

type CartListing struct {
	SellerID int32
	Status   string
}
