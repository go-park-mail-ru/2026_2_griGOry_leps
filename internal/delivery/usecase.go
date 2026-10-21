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

type AdUsecase interface {
	List(ctx context.Context, p domain.ListAdsParams) (domain.AdPage, error)
}
