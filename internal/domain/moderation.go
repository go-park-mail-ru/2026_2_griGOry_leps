package domain

import (
	"errors"
	"time"
)

const MaxRejectionReason = 500

var (
	ErrModerationConflict     = errors.New("listing changed or is no longer awaiting moderation")
	ErrInvalidRevision        = errors.New("revision must be positive")
	ErrInvalidRejectionReason = errors.New("reason must be 1-500 characters")
	ErrOwnModeration          = errors.New("cannot moderate your own listing")
)

type ModerationRequest struct {
	ID          int32
	Title       string
	Price       *int64
	RequestedAt time.Time
}

type ModerationPage struct {
	Items      []ModerationRequest
	NextCursor string
}

type ModerationSnapshot struct {
	Listing     Listing
	Revision    int64
	RequestedAt time.Time
}
