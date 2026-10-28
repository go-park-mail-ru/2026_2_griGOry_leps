package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

var errTwoDefaults = errors.New("two default addresses")

type addressRepoFake struct {
	lastID int32
	items  []domain.Address
	locked []int32
}

func (r *addressRepoFake) LockByID(_ context.Context, id int32) error {
	r.locked = append(r.locked, id)
	return nil
}

func (r *addressRepoFake) List(_ context.Context, userID int32) ([]domain.Address, error) {
	var out []domain.Address
	for _, a := range r.items {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (r *addressRepoFake) Count(ctx context.Context, userID int32) (int, error) {
	list, _ := r.List(ctx, userID)
	return len(list), nil
}

func (r *addressRepoFake) index(userID, id int32) int {
	for i, a := range r.items {
		if a.ID == id && a.UserID == userID {
			return i
		}
	}
	return -1
}

func (r *addressRepoFake) GetForUpdate(_ context.Context, userID, id int32) (domain.Address, error) {
	i := r.index(userID, id)
	if i < 0 {
		return domain.Address{}, domain.ErrAddressNotFound
	}
	return r.items[i], nil
}

func (r *addressRepoFake) Create(_ context.Context, a domain.Address) (domain.Address, error) {
	r.lastID++
	a.ID = r.lastID
	a.CreatedAt = time.Unix(int64(a.ID), 0)
	r.items = append(r.items, a)
	return a, r.checkDefaults(a.UserID)
}

func (r *addressRepoFake) Update(_ context.Context, a domain.Address) (domain.Address, error) {
	i := r.index(a.UserID, a.ID)
	if i < 0 {
		return domain.Address{}, domain.ErrAddressNotFound
	}
	r.items[i] = a
	return a, r.checkDefaults(a.UserID)
}

func (r *addressRepoFake) Delete(_ context.Context, userID, id int32) error {
	i := r.index(userID, id)
	if i < 0 {
		return domain.ErrAddressNotFound
	}
	r.items = append(r.items[:i], r.items[i+1:]...)
	return nil
}

func (r *addressRepoFake) ClearDefault(_ context.Context, userID int32) error {
	for i := range r.items {
		if r.items[i].UserID == userID {
			r.items[i].IsDefault = false
		}
	}
	return nil
}

func (r *addressRepoFake) PromoteOldest(_ context.Context, userID, exceptID int32) error {
	for i := range r.items {
		if r.items[i].UserID == userID && r.items[i].ID != exceptID {
			r.items[i].IsDefault = true
			return r.checkDefaults(userID)
		}
	}
	return nil
}

func (r *addressRepoFake) checkDefaults(userID int32) error {
	count := 0
	for _, a := range r.items {
		if a.UserID == userID && a.IsDefault {
			count++
		}
	}
	if count > 1 {
		return errTwoDefaults
	}
	return nil
}

func (r *addressRepoFake) defaultID(userID int32) int32 {
	for _, a := range r.items {
		if a.UserID == userID && a.IsDefault {
			return a.ID
		}
	}
	return 0
}

func newAddressUsecase() (*AddressUsecase, *addressRepoFake) {
	repo := &addressRepoFake{}
	return NewAddressUsecase(repo, repo, txFake{}), repo
}

func str(s string) *string { return &s }

func flag(b bool) *bool { return &b }

func address(street string, isDefault bool) domain.AddressInput {
	return domain.AddressInput{Street: str(street), House: str("д. 1"), Comment: str(""), IsDefault: flag(isDefault)}
}

func TestAddressUsecase_FirstAddressIsDefault(t *testing.T) {
	uc, repo := newAddressUsecase()

	a, err := uc.Create(context.Background(), 7, address("  Тверская  ", false))
	require.NoError(t, err)
	require.True(t, a.IsDefault)
	require.Equal(t, "Тверская", a.Street)
	require.Equal(t, []int32{7}, repo.locked)
}

func TestAddressUsecase_CreateDefaultMovesFlag(t *testing.T) {
	uc, repo := newAddressUsecase()
	ctx := context.Background()

	first, err := uc.Create(ctx, 7, address("Тверская", true))
	require.NoError(t, err)
	second, err := uc.Create(ctx, 7, address("Ленинский", false))
	require.NoError(t, err)
	require.False(t, second.IsDefault)
	require.Equal(t, first.ID, repo.defaultID(7))

	third, err := uc.Create(ctx, 7, address("Арбат", true))
	require.NoError(t, err)
	require.Equal(t, third.ID, repo.defaultID(7))
}

func TestAddressUsecase_DeleteDefaultPromotesOldest(t *testing.T) {
	uc, repo := newAddressUsecase()
	ctx := context.Background()

	_, _ = uc.Create(ctx, 7, address("Тверская", false))
	second, _ := uc.Create(ctx, 7, address("Ленинский", false))
	third, _ := uc.Create(ctx, 7, address("Арбат", true))

	require.NoError(t, uc.Delete(ctx, 7, third.ID))
	require.Equal(t, int32(1), repo.defaultID(7))

	require.NoError(t, uc.Delete(ctx, 7, 1))
	require.Equal(t, second.ID, repo.defaultID(7))

	require.NoError(t, uc.Delete(ctx, 7, second.ID))
	require.Zero(t, repo.defaultID(7))
}

func TestAddressUsecase_DeleteForeignAddress(t *testing.T) {
	uc, _ := newAddressUsecase()
	a, _ := uc.Create(context.Background(), 7, address("Тверская", false))

	require.ErrorIs(t, uc.Delete(context.Background(), 8, a.ID), domain.ErrAddressNotFound)
}

func TestAddressUsecase_UpdateDefault(t *testing.T) {
	uc, repo := newAddressUsecase()
	ctx := context.Background()

	first, _ := uc.Create(ctx, 7, address("Тверская", false))
	second, _ := uc.Create(ctx, 7, address("Ленинский", false))

	updated, err := uc.Update(ctx, 7, second.ID, domain.AddressInput{IsDefault: flag(true)})
	require.NoError(t, err)
	require.True(t, updated.IsDefault)
	require.Equal(t, "Ленинский", updated.Street)
	require.Equal(t, second.ID, repo.defaultID(7))

	updated, err = uc.Update(ctx, 7, second.ID, domain.AddressInput{IsDefault: flag(false), Comment: str(" Домофон 34 ")})
	require.NoError(t, err)
	require.False(t, updated.IsDefault)
	require.Equal(t, "Домофон 34", updated.Comment)
	require.Equal(t, first.ID, repo.defaultID(7))
}

func TestAddressUsecase_OnlyAddressStaysDefault(t *testing.T) {
	uc, _ := newAddressUsecase()
	a, _ := uc.Create(context.Background(), 7, address("Тверская", false))

	updated, err := uc.Update(context.Background(), 7, a.ID, domain.AddressInput{IsDefault: flag(false)})
	require.NoError(t, err)
	require.True(t, updated.IsDefault)
}

func TestAddressUsecase_Validation(t *testing.T) {
	tests := []struct {
		name string
		in   domain.AddressInput
		want error
	}{
		{"no street", domain.AddressInput{House: str("д. 1")}, domain.ErrMissingStreet},
		{"blank street", domain.AddressInput{Street: str("  "), House: str("д. 1")}, domain.ErrMissingStreet},
		{"long street", domain.AddressInput{Street: str(strings.Repeat("я", maxStreetLength+1)), House: str("д. 1")}, domain.ErrStreetTooLong},
		{"no house", domain.AddressInput{Street: str("Тверская")}, domain.ErrMissingHouse},
		{"long house", domain.AddressInput{Street: str("Тверская"), House: str(strings.Repeat("я", maxHouseLength+1))}, domain.ErrHouseTooLong},
		{"long comment", domain.AddressInput{Street: str("Тверская"), House: str("д. 1"),
			Comment: str(strings.Repeat("я", maxAddressCommentLength+1))}, domain.ErrAddressCommentTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, repo := newAddressUsecase()
			_, err := uc.Create(context.Background(), 7, tt.in)
			require.ErrorIs(t, err, tt.want)
			require.Empty(t, repo.items)
		})
	}
}

func TestAddressUsecase_UpdateValidatesOnlyPassedFields(t *testing.T) {
	uc, _ := newAddressUsecase()
	a, _ := uc.Create(context.Background(), 7, address("Тверская", false))

	_, err := uc.Update(context.Background(), 7, a.ID, domain.AddressInput{House: str(" ")})
	require.ErrorIs(t, err, domain.ErrMissingHouse)

	_, err = uc.Update(context.Background(), 7, a.ID+1, domain.AddressInput{})
	require.ErrorIs(t, err, domain.ErrAddressNotFound)
}
