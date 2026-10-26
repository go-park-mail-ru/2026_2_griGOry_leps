package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"
)

const (
	AdStatusDraft      = "draft"
	AdStatusModeration = "moderation"
	AdStatusClosed     = "closed"
	AdStatusSold       = "sold"

	MaxListingPhotos = 10
)

var (
	ErrListingNotFound    = errors.New("listing not found")
	ErrForbidden          = errors.New("forbidden")
	ErrListingSold        = errors.New("listing is sold")
	ErrListingNotDraft    = errors.New("listing is not a draft")
	ErrListingNotActive   = errors.New("listing is not active")
	ErrInvalidCloseReason = errors.New("invalid close reason")

	ErrInvalidCategory    = errors.New("invalid category_id")
	ErrInvalidTitle       = errors.New("title must be 1-80 characters")
	ErrPhotosRequired     = errors.New("at least one photo is required")
	ErrInvalidPhotoID     = errors.New("invalid photo id")
	ErrTooManyPhotos      = errors.New("at most 10 photos are allowed")
	ErrDescriptionTooLong = errors.New("description must be at most 3000 characters")
	ErrInvalidCondition   = errors.New("invalid condition")
	ErrAddressRequired    = errors.New("address is required")
	ErrAddressTooLong     = errors.New("address must be at most 255 characters")
)

type Optional[T any] struct {
	Set   bool
	Null  bool
	Value T
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(data, []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}

type ListingFields struct {
	CategoryID   Optional[int32]
	Title        *string
	PhotoIDs     *[]int32
	Description  *string
	Price        Optional[string]
	IsFree       *bool
	IsNegotiable *bool
	Condition    Optional[string]
	Address      Optional[string]
	HasDelivery  *bool
}

type ListingState struct {
	UserID       int32
	Status       string
	CategoryID   *int32
	Title        string
	PhotoIDs     []int32
	Description  string
	Price        *int64
	IsFree       bool
	IsNegotiable bool
	Condition    *string
	Address      *string
	HasDelivery  bool
}

type Category struct {
	ID       int32
	ParentID *int32
	Name     string
	Slug     string
}

type CategoryNode struct {
	ID       int32
	Name     string
	Slug     string
	Children []CategoryNode
}

type ListingCategory struct {
	ID   int32
	Name string
	Path string
}

type ListingPhoto struct {
	ID  int32
	URL string
}

type Seller struct {
	ID           int32
	FirstName    string
	Avatar       string
	Rating       float64
	ReviewsCount int
	CreatedAt    time.Time
}

type Listing struct {
	ID           int32
	Category     *ListingCategory
	Title        string
	Description  string
	Photos       []ListingPhoto
	Price        *int64
	IsFree       bool
	IsNegotiable bool
	Condition    *string
	City         string
	Address      *string
	HasDelivery  bool
	Status       string
	CloseReason  *string
	Seller       Seller
	IsOwner      bool
	InCart       bool
	PublishedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
