package usecase

import (
	"context"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, email, passwordHash, firstName, phone string) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetByPhone(ctx context.Context, phone string) (domain.User, error)
	GetByID(ctx context.Context, id int32) (domain.User, error)
	GetProfile(ctx context.Context, id int32) (domain.Profile, error)
	UpdateProfile(ctx context.Context, id int32, upd domain.ProfileUpdate) error
	LockByID(ctx context.Context, id int32) error
	GetPasswordHashForUpdate(ctx context.Context, id int32) (string, error)
	UpdatePassword(ctx context.Context, id int32, passwordHash string) error
	SetEmailVerified(ctx context.Context, id int32) error
	SetPendingEmail(ctx context.Context, id int32, email *string) error
	ConfirmPendingEmail(ctx context.Context, id int32) error
	GetAvatarForUpdate(ctx context.Context, id int32) (string, error)
	UpdateAvatar(ctx context.Context, id int32, avatar string) error
}

type SessionRepository interface {
	Create(ctx context.Context, id string, userID int32, expiresAt time.Time) error
	GetByID(ctx context.Context, id string) (domain.Session, error)
	Delete(ctx context.Context, id string) error
	DeleteOthers(ctx context.Context, userID int32, keepID string) error
	DeleteAll(ctx context.Context, userID int32) error
}

type PasswordResetRepository interface {
	Create(ctx context.Context, tokenHash string, userID int32, expiresAt time.Time) error
	HasCreatedSince(ctx context.Context, userID int32, since time.Time) (bool, error)
	Get(ctx context.Context, tokenHash string) (domain.PasswordReset, error)
	GetForUpdate(ctx context.Context, tokenHash string) (domain.PasswordReset, error)
	UseAll(ctx context.Context, userID int32) error
}

type EmailVerificationRepository interface {
	Create(ctx context.Context, tokenHash string, userID int32, email string, expiresAt time.Time) error
	LastCreatedAt(ctx context.Context, userID int32) (time.Time, error)
	Get(ctx context.Context, tokenHash string) (domain.EmailVerification, error)
	GetForUpdate(ctx context.Context, tokenHash string) (domain.EmailVerification, error)
	UseAll(ctx context.Context, userID int32) error
}

type Mailer interface {
	SendPasswordReset(ctx context.Context, to, link string) error
	SendEmailVerification(ctx context.Context, to, link string) error
}

type EmailVerifier interface {
	SendLink(ctx context.Context, userID int32, email string) error
}

type FileStorage interface {
	Save(ctx context.Context, name string, data []byte) error
	Delete(ctx context.Context, name string) error
}

type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type AdRepository interface {
	List(ctx context.Context, f domain.AdFilter) ([]domain.Ad, int, error)
	ListByUser(ctx context.Context, f domain.MyAdFilter) ([]domain.Ad, map[string]int, error)
}

type SellerRepository interface {
	Get(ctx context.Context, id int32) (domain.SellerProfile, error)
	ListAds(ctx context.Context, f domain.SellerAdFilter) (domain.SellerAdPage, error)
	ListReviews(ctx context.Context, f domain.ReviewFilter) (domain.ReviewPage, error)
	CreateReport(ctx context.Context, report domain.SellerReport) error
}

type AddressRepository interface {
	List(ctx context.Context, userID int32) ([]domain.Address, error)
	Count(ctx context.Context, userID int32) (int, error)
	GetForUpdate(ctx context.Context, userID, id int32) (domain.Address, error)
	Create(ctx context.Context, a domain.Address) (domain.Address, error)
	Update(ctx context.Context, a domain.Address) (domain.Address, error)
	Delete(ctx context.Context, userID, id int32) error
	ClearDefault(ctx context.Context, userID int32) error
	PromoteOldest(ctx context.Context, userID, exceptID int32) error
}

type CityRepository interface {
	Search(ctx context.Context, prefix string, limit int) ([]domain.City, error)
}

type UserLocker interface {
	LockByID(ctx context.Context, id int32) error
}

type ListingRepository interface {
	Get(ctx context.Context, id, viewerID int32) (domain.Listing, error)
	GetStateForUpdate(ctx context.Context, id int32) (domain.ListingState, error)
	Create(ctx context.Context, s domain.ListingState) (int32, error)
	Update(ctx context.Context, id int32, s domain.ListingState) error
	SetPhotos(ctx context.Context, userID, listingID int32, photoIDs []int32) ([]string, error)
	Delete(ctx context.Context, id int32) ([]string, error)
	Close(ctx context.Context, id int32, reason string) error
	IsLeafCategory(ctx context.Context, id int32) (bool, error)
	ListCategories(ctx context.Context) ([]domain.Category, error)
	CreatePhoto(ctx context.Context, userID int32, url string) (int32, error)
	ApproveModeration(ctx context.Context) (int64, error)
	DeleteStalePhotos(ctx context.Context, before time.Time) ([]string, error)
}
