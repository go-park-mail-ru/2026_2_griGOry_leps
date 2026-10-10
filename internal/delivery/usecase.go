package delivery

import (
	"context"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type AuthUsecase interface {
	Register(ctx context.Context, email, password, firstName, nickname, phone string) (domain.User, domain.Session, error)
	Login(ctx context.Context, login, password string) (domain.User, domain.Session, error)
	Logout(ctx context.Context, sessionID string) error
	Me(ctx context.Context, sessionID string) (domain.User, error)
}

type AdUsecase interface {
	List(ctx context.Context, p domain.ListAdsParams) (domain.AdPage, error)
}
