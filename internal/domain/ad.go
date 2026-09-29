package domain

import "time"

type AdCard struct {
	ID           int32
	Title        string
	Price        string
	ImageURL     *string
	City         *string
	HasDelivery  bool
	CategoryID   *int32
	Rating       *float64
	ReviewsCount int
	IsFavorite   bool
	CreatedAt    time.Time
}

type AdPage struct {
	Items  []AdCard
	Total  int
	Limit  int
	Offset int
}
