package domain

import (
	"errors"
	"time"
)

var (
	ErrModerationConflict     = errors.New("listing changed or is no longer awaiting moderation")
	ErrInvalidRevision        = errors.New("revision must be positive")
	ErrInvalidRejectionReason = errors.New("reason must be 1-500 characters")
)

type ModerationItem struct {
	ID        int32
	Title     string
	Revision  int64
	UpdatedAt time.Time
}

type ModerationListing struct {
	Listing  Listing
	Revision int64
}
