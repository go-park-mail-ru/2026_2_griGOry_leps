package domain

import "time"

type Address struct {
	ID        int32
	UserID    int32
	Street    string
	House     string
	Comment   string
	IsDefault bool
	CreatedAt time.Time
}

type AddressInput struct {
	Street    *string
	House     *string
	Comment   *string
	IsDefault *bool
}
