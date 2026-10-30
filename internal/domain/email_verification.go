package domain

import "time"

type EmailVerification struct {
	UserID    int32
	Email     string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

func (v EmailVerification) Valid(now time.Time) bool {
	return v.UsedAt == nil && now.Before(v.ExpiresAt)
}

type EmailVerifyResult struct {
	Email   string
	Changed bool
}
