package domain

import "errors"

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("user already exists")
	ErrPhoneExists       = errors.New("phone already registered")
	ErrNicknameExists    = errors.New("nickname already taken")
	ErrSessionNotFound   = errors.New("session not found")
	ErrInvalidEmail      = errors.New("invalid email")
	ErrMissingFirstName  = errors.New("first name is required")
	ErrFirstNameTooLong  = errors.New("first name is too long")
	ErrMissingNickname   = errors.New("nickname is required")
	ErrInvalidNickname   = errors.New("nickname must be 3-32 characters: latin letters, digits, _ and .")
	ErrInvalidPhone      = errors.New("invalid phone number")
	ErrWeakPassword      = errors.New("password must be at least 8 characters and contain uppercase, lowercase letters and a digit")
	ErrPasswordTooLong   = errors.New("password is too long")
	ErrEmailTaken        = errors.New("email already registered")
	ErrPhoneTaken        = errors.New("phone already registered")
	ErrNicknameTaken     = errors.New("nickname already taken")
	ErrInvalidLogin      = errors.New("invalid login or password")
	ErrSessionExpired    = errors.New("session expired")
	ErrInvalidPrice      = errors.New("invalid price")
	ErrInvalidPriceRange = errors.New("price_max must not be less than price_min")
	ErrInvalidSort       = errors.New("invalid sort value")
)
