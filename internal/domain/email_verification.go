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

// CooldownError — письмо со ссылкой отправляли недавно, новое можно через Wait.
type CooldownError struct {
	Wait time.Duration
}

func (e *CooldownError) Error() string { return "too many requests" }
