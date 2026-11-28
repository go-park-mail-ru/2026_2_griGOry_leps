package usecase

import (
	"context"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type OnlineConfAccessUsecase struct {
	admins OnlineConfAdminRepository
}

func NewOnlineConfAccessUsecase(admins OnlineConfAdminRepository) *OnlineConfAccessUsecase {
	return &OnlineConfAccessUsecase{admins: admins}
}

func (u *OnlineConfAccessUsecase) CheckAdmin(ctx context.Context, userID int32) error {
	if userID <= 0 {
		return domain.ErrForbidden
	}
	admin, err := u.admins.IsAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !admin {
		return domain.ErrForbidden
	}
	return nil
}

func (u *OnlineConfAccessUsecase) GrantAdmin(ctx context.Context, userID int32, grantedBy *int32) (bool, error) {
	if userID <= 0 {
		return false, domain.ErrInvalidUserID
	}
	return u.admins.Grant(ctx, userID, grantedBy)
}
