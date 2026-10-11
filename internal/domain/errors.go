package domain

import "errors"

var (
	ErrInvalidPrice      = errors.New("invalid price")
	ErrInvalidPriceRange = errors.New("price_max must not be less than price_min")
	ErrInvalidSort       = errors.New("invalid sort value")
	ErrInvalidEmail      = errors.New("invalid email")
	ErrMissingFirstName  = errors.New("first name is required")
	ErrInvalidFirstName  = errors.New("first name must be 2-50 characters: latin or cyrillic letters, digits, spaces, -, _ and .")
	ErrInvalidPhone      = errors.New("invalid phone number")
	ErrWeakPassword      = errors.New("password must be at least 8 characters and contain uppercase, lowercase letters and a digit")
	ErrPasswordTooLong   = errors.New("password is too long")
	ErrEmailTaken        = errors.New("email already registered")
	ErrPhoneTaken        = errors.New("phone already registered")
	ErrInvalidLogin      = errors.New("invalid login or password")
	ErrSessionExpired    = errors.New("session expired")
	ErrSessionNotFound   = errors.New("session not found")
	ErrUserNotFound      = errors.New("user not found")
)
