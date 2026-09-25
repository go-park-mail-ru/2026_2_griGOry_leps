package domain

import "time"

type User struct {
	ID           int32
	Nickname     string
	FirstName    string
	LastName     string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}
