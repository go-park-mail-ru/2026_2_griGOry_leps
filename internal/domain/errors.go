package domain

import "errors"

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrUserExists      = errors.New("user already exists")
	ErrPhoneExists     = errors.New("phone already registered")
	ErrNicknameExists  = errors.New("nickname already taken")
	ErrSessionNotFound = errors.New("session not found")
)
