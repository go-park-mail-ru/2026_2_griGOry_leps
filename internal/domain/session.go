package domain

import "time"

type Session struct {
	ID        string
	UserID    int32
	ExpiresAt time.Time
}
