package domain

import "time"

type User struct {
	ID           int32
	FirstName    string
	Email        string
	Phone        string
	PasswordHash string
	CreatedAt    time.Time
}

type City struct {
	ID   int32
	Name string
}

type Profile struct {
	User
	PhoneVerified bool
	City          *City
	About         string
	Avatar        string // имя файла, пусто — аватарки нет
	Rating        float64
	ReviewsCount  int
}

// nil — поле не меняется
type ProfileUpdate struct {
	FirstName *string
	Phone     *string
	Email     *string
	CityID    *int32
	About     *string
}
