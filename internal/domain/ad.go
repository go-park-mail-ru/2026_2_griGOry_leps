package domain

import "time"

const (
	AdStatusActive   = "active"
	AdStatusRejected = "rejected"

	AdSortNewest    = "newest"
	AdSortPriceAsc  = "price_asc"
	AdSortPriceDesc = "price_desc"
)

type Ad struct {
	ID          int32
	UserID      int32
	CategoryID  int32
	Title       string
	Description string
	Price       *int64 // в копейках
	City        string
	HasDelivery bool
	IsFree      bool
	ImageURL    string
	Status      string
	CloseReason string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AdFilter struct {
	Query       string
	CategoryID  int32
	City        string
	PriceMin    *int64
	PriceMax    *int64
	HasDelivery *bool
	Sort        string
	Limit       int
	Offset      int
}

type AdPage struct {
	Items  []Ad
	Total  int
	Limit  int
	Offset int
}

var AdStatuses = []string{
	AdStatusDraft, AdStatusModeration, AdStatusRejected, AdStatusActive, AdStatusClosed, AdStatusSold,
}

type MyAdFilter struct {
	UserID int32
	Status string
	Limit  int
	Offset int
}

type MyAdPage struct {
	AdPage
	Counts map[string]int
}

type ListAdsParams struct {
	Query       string
	CategoryID  int32
	City        string
	PriceMin    string
	PriceMax    string
	HasDelivery *bool
	Sort        string
	Limit       int
	Offset      int
}
