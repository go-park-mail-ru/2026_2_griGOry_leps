package domain

import "time"

type User struct {
	ID           int32
	Nickname     string
	Firstname    string
	Secondname   string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}