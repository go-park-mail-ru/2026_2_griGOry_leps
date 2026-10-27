package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	maxStreetLength         = 200
	maxHouseLength          = 50
	maxAddressCommentLength = 200
)

type AddressUsecase struct {
	addresses AddressRepository
	users     UserLocker
	tx        Transactor
}

func NewAddressUsecase(addresses AddressRepository, users UserLocker, tx Transactor) *AddressUsecase {
	return &AddressUsecase{addresses: addresses, users: users, tx: tx}
}

func (uc *AddressUsecase) List(ctx context.Context, userID int32) ([]domain.Address, error) {
	return uc.addresses.List(ctx, userID)
}

func normalizeAddressInput(in *domain.AddressInput, create bool) error {
	if create {
		if in.Street == nil {
			return domain.ErrMissingStreet
		}
		if in.House == nil {
			return domain.ErrMissingHouse
		}
	}

	if in.Street != nil {
		street := strings.TrimSpace(*in.Street)
		switch {
		case street == "":
			return domain.ErrMissingStreet
		case utf8.RuneCountInString(street) > maxStreetLength:
			return domain.ErrStreetTooLong
		}
		in.Street = &street
	}

	if in.House != nil {
		house := strings.TrimSpace(*in.House)
		switch {
		case house == "":
			return domain.ErrMissingHouse
		case utf8.RuneCountInString(house) > maxHouseLength:
			return domain.ErrHouseTooLong
		}
		in.House = &house
	}

	if in.Comment != nil {
		comment := strings.TrimSpace(*in.Comment)
		if utf8.RuneCountInString(comment) > maxAddressCommentLength {
			return domain.ErrAddressCommentTooLong
		}
		in.Comment = &comment
	}
	return nil
}

func (uc *AddressUsecase) Create(ctx context.Context, userID int32, in domain.AddressInput) (domain.Address, error) {
	if err := normalizeAddressInput(&in, true); err != nil {
		return domain.Address{}, err
	}

	a := domain.Address{UserID: userID, Street: *in.Street, House: *in.House}
	if in.Comment != nil {
		a.Comment = *in.Comment
	}

	var created domain.Address
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.users.LockByID(ctx, userID); err != nil {
			return err
		}
		count, err := uc.addresses.Count(ctx, userID)
		if err != nil {
			return err
		}

		a.IsDefault = count == 0 || (in.IsDefault != nil && *in.IsDefault)
		if a.IsDefault {
			if err := uc.addresses.ClearDefault(ctx, userID); err != nil {
				return err
			}
		}
		created, err = uc.addresses.Create(ctx, a)
		return err
	})
	return created, err
}

func (uc *AddressUsecase) Update(ctx context.Context, userID, id int32, in domain.AddressInput) (domain.Address, error) {
	if err := normalizeAddressInput(&in, false); err != nil {
		return domain.Address{}, err
	}

	var updated domain.Address
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.users.LockByID(ctx, userID); err != nil {
			return err
		}
		a, err := uc.addresses.GetForUpdate(ctx, userID, id)
		if err != nil {
			return err
		}

		if in.Street != nil {
			a.Street = *in.Street
		}
		if in.House != nil {
			a.House = *in.House
		}
		if in.Comment != nil {
			a.Comment = *in.Comment
		}

		promote := false
		switch {
		case in.IsDefault == nil || *in.IsDefault == a.IsDefault:
		case *in.IsDefault:
			if err := uc.addresses.ClearDefault(ctx, userID); err != nil {
				return err
			}
			a.IsDefault = true
		default:
			count, err := uc.addresses.Count(ctx, userID)
			if err != nil {
				return err
			}
			if count > 1 {
				a.IsDefault = false
				promote = true
			}
		}

		if updated, err = uc.addresses.Update(ctx, a); err != nil {
			return err
		}
		if promote {
			return uc.addresses.PromoteOldest(ctx, userID, id)
		}
		return nil
	})
	return updated, err
}

func (uc *AddressUsecase) Delete(ctx context.Context, userID, id int32) error {
	return uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.users.LockByID(ctx, userID); err != nil {
			return err
		}
		a, err := uc.addresses.GetForUpdate(ctx, userID, id)
		if err != nil {
			return err
		}
		if err := uc.addresses.Delete(ctx, userID, id); err != nil {
			return err
		}
		if a.IsDefault {
			return uc.addresses.PromoteOldest(ctx, userID, id)
		}
		return nil
	})
}
