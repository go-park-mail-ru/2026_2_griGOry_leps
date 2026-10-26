package domain

import "time"

type PasswordReset struct {
	UserID    int32
	ExpiresAt time.Time
	UsedAt    *time.Time
}

func (r PasswordReset) Valid(now time.Time) bool {
	return r.UsedAt == nil && now.Before(r.ExpiresAt)
}
