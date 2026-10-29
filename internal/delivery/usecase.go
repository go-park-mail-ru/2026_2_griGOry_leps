package delivery

import (
	"context"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type AuthUsecase interface {
	Register(ctx context.Context, email, password, firstName, phone string) (domain.User, domain.Session, error)
	Login(ctx context.Context, login, password string) (domain.User, domain.Session, error)
	Logout(ctx context.Context, sessionID string) error
	Me(ctx context.Context, sessionID string) (domain.User, error)
}

type ProfileUsecase interface {
	Profile(ctx context.Context, userID int32) (domain.Profile, error)
	UpdateProfile(ctx context.Context, userID int32, upd domain.ProfileUpdate) (domain.Profile, error)
	ChangePassword(ctx context.Context, userID int32, sessionID, current, next string) error
	UploadAvatar(ctx context.Context, userID int32, data []byte) (string, error)
}

type PasswordResetUsecase interface {
	RequestReset(ctx context.Context, login string) error
	CheckToken(ctx context.Context, token string) (string, error)
	ResetPassword(ctx context.Context, token, password string) error
}

type AdUsecase interface {
	List(ctx context.Context, p domain.ListAdsParams) (domain.AdPage, error)
	ListMine(ctx context.Context, f domain.MyAdFilter) (domain.MyAdPage, error)
}

type SellerUsecase interface {
	Profile(ctx context.Context, id int32) (domain.SellerProfile, error)
	Listings(ctx context.Context, f domain.SellerAdFilter) (domain.SellerAdPage, error)
	Reviews(ctx context.Context, f domain.ReviewFilter) (domain.ReviewPage, error)
	Report(ctx context.Context, report domain.SellerReport) error
}

type AddressUsecase interface {
	List(ctx context.Context, userID int32) ([]domain.Address, error)
	Create(ctx context.Context, userID int32, in domain.AddressInput) (domain.Address, error)
	Update(ctx context.Context, userID, id int32, in domain.AddressInput) (domain.Address, error)
	Delete(ctx context.Context, userID, id int32) error
}

type CityUsecase interface {
	Search(ctx context.Context, query string) ([]domain.City, error)
}

type ListingUsecase interface {
	Get(ctx context.Context, id, viewerID int32) (domain.Listing, error)
	Create(ctx context.Context, userID int32, f domain.ListingFields, publish bool) (domain.Listing, error)
	Update(ctx context.Context, userID, id int32, f domain.ListingFields, publish *bool) (domain.Listing, error)
	Delete(ctx context.Context, userID, id int32) error
	Close(ctx context.Context, userID, id int32, reason string) (domain.Listing, error)
	UploadPhoto(ctx context.Context, userID int32, data []byte) (domain.ListingPhoto, error)
	Categories(ctx context.Context) ([]domain.CategoryNode, error)
}
