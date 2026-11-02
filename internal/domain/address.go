package domain

import "time"

type Address struct {
	ID        int32
	UserID    int32
	City      string
	Street    string
	House     string
	Apartment string
	Comment   string
	IsDefault bool
	CreatedAt time.Time
}

type AddressInput struct {
	City      *string
	Street    *string
	House     *string
	Apartment *string
	Comment   *string
	IsDefault *bool
}
