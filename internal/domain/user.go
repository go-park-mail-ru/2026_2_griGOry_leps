package domain

import "time"

type User struct {
	ID           int32
	FirstName    string
	Nickname     string
	Email        string
	Phone        string
	PasswordHash string
	CreatedAt    time.Time
}
