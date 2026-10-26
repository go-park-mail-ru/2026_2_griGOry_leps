package domain

import "time"

const (
	ReviewSortNewest     = "newest"
	ReviewSortRatingDesc = "rating_desc"
	ReviewSortRatingAsc  = "rating_asc"

	ReportReasonFraud      = "fraud"
	ReportReasonProhibited = "prohibited"
	ReportReasonRude       = "rude"
	ReportReasonOther      = "other"
)

type SellerShort struct {
	ID           int32
	FirstName    string
	Avatar       string
	Rating       float64
	ReviewsCount int
	CreatedAt    time.Time
}

type SellerProfile struct {
	SellerShort
	Location            string
	PhoneVerified       bool
	CompletedSalesCount int
	ActiveListingsCount int
}

type SellerAdFilter struct {
	SellerID   int32
	CategoryID int32
	Sort       string
	Limit      int
	Offset     int
}

type SellerCategory struct {
	ID    int32
	Name  string
	Count int
}

type SellerAdPage struct {
	AdPage
	Categories []SellerCategory
}

type ReviewAuthor struct {
	ID        int32
	FirstName string
	Avatar    string
}

type Review struct {
	ID           int32
	Author       ReviewAuthor
	Rating       float64
	Text         string
	ListingTitle string
	Photos       []string
	CreatedAt    time.Time
}

type ReviewFilter struct {
	SellerID   int32
	Sort       string
	WithPhotos bool
	Limit      int
	Offset     int
}

type ReviewSummary struct {
	Rating       float64
	Distribution map[int]int
}

type ReviewPage struct {
	Items  []Review
	Total  int
	Limit  int
	Offset int
	ReviewSummary
}

type SellerReport struct {
	SellerID   int32
	ReporterID int32
	Reason     string
	Comment    string
}
