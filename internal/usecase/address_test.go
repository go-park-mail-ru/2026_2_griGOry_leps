package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type addressTestEnv struct {
	*AddressUsecase
	repo  *MockAddressRepository
	users *MockUserLocker
	tx    *MockTransactor
}

func newAddressTestEnv(t *testing.T) *addressTestEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	e := &addressTestEnv{repo: NewMockAddressRepository(ctrl), users: NewMockUserLocker(ctrl), tx: NewMockTransactor(ctrl)}
	e.AddressUsecase = NewAddressUsecase(e.repo, e.users, e.tx)
	return e
}

func expectAddressLock(e *addressTestEnv, userID int32) *gomock.Call {
	expectTransaction(e.tx)
	return e.users.EXPECT().LockByID(gomock.Any(), userID).Return(nil)
}

func str(s string) *string { return &s }

func flag(b bool) *bool { return &b }

func address(street string, isDefault bool) domain.AddressInput {
	return domain.AddressInput{City: str("Москва"), Street: str(street), House: str("д. 1"), Comment: str(""), IsDefault: flag(isDefault)}
}

func TestAddressUsecase_FirstAddressIsDefault(t *testing.T) {
	e := newAddressTestEnv(t)
	lock := expectAddressLock(e, 7)
	gomock.InOrder(
		lock,
		e.repo.EXPECT().Count(gomock.Any(), int32(7)).Return(0, nil),
		e.repo.EXPECT().ClearDefault(gomock.Any(), int32(7)).Return(nil),
		e.repo.EXPECT().Create(gomock.Any(), domain.Address{UserID: 7, City: "Москва", Street: "Тверская", House: "д. 1", IsDefault: true}).Return(domain.Address{ID: 1, IsDefault: true}, nil),
	)
	a, err := e.Create(context.Background(), 7, address("  Тверская  ", false))
	require.NoError(t, err)
	require.True(t, a.IsDefault)
}

func TestAddressUsecase_CityAndApartment(t *testing.T) {
	e := newAddressTestEnv(t)
	a := domain.Address{UserID: 7, City: "Химки", Street: "Тверская", House: "д. 1", Apartment: "12", IsDefault: true}
	expectAddressLock(e, 7)
	e.repo.EXPECT().Count(gomock.Any(), int32(7)).Return(0, nil)
	e.repo.EXPECT().ClearDefault(gomock.Any(), int32(7)).Return(nil)
	created := a
	created.ID = 1
	e.repo.EXPECT().Create(gomock.Any(), a).Return(created, nil)
	in := address("Тверская", false)
	in.City = str("  Химки ")
	in.Apartment = str(" 12 ")
	got, err := e.Create(context.Background(), 7, in)
	require.NoError(t, err)
	require.Equal(t, created, got)
	expectAddressLock(e, 7)
	e.repo.EXPECT().GetForUpdate(gomock.Any(), int32(7), int32(1)).Return(created, nil)
	updated := created
	updated.Apartment = ""
	e.repo.EXPECT().Update(gomock.Any(), updated).Return(updated, nil)
	got, err = e.Update(context.Background(), 7, 1, domain.AddressInput{Apartment: str("")})
	require.NoError(t, err)
	require.Equal(t, updated, got)
	_, err = e.Update(context.Background(), 7, 1, domain.AddressInput{City: str(" ")})
	require.ErrorIs(t, err, domain.ErrMissingCity)
}

func TestAddressUsecase_CreateDefaultMovesFlag(t *testing.T) {
	for _, makeDefault := range []bool{false, true} {
		t.Run(fmt.Sprint(makeDefault), func(t *testing.T) {
			e := newAddressTestEnv(t)
			lock := expectAddressLock(e, 7)
			count := e.repo.EXPECT().Count(gomock.Any(), int32(7)).Return(2, nil).After(lock)
			if makeDefault {
				count = e.repo.EXPECT().ClearDefault(gomock.Any(), int32(7)).Return(nil).After(count)
			}
			a := domain.Address{UserID: 7, City: "Москва", Street: "street", House: "д. 1", IsDefault: makeDefault}
			e.repo.EXPECT().Create(gomock.Any(), a).Return(a, nil).After(count)
			got, err := e.Create(context.Background(), 7, address("street", makeDefault))
			require.NoError(t, err)
			require.Equal(t, makeDefault, got.IsDefault)
		})
	}
}

func TestAddressUsecase_DeleteDefaultPromotesOldest(t *testing.T) {
	for _, id := range []int32{3, 1, 2} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			e := newAddressTestEnv(t)
			lock := expectAddressLock(e, 7)
			gomock.InOrder(
				lock,
				e.repo.EXPECT().GetForUpdate(gomock.Any(), int32(7), id).Return(domain.Address{ID: id, IsDefault: true}, nil),
				e.repo.EXPECT().Delete(gomock.Any(), int32(7), id).Return(nil),
				e.repo.EXPECT().PromoteOldest(gomock.Any(), int32(7), id).Return(nil),
			)
			require.NoError(t, e.Delete(context.Background(), 7, id))
		})
	}
}

func TestAddressUsecase_DeleteForeignAddress(t *testing.T) {
	e := newAddressTestEnv(t)
	expectAddressLock(e, 8)
	e.repo.EXPECT().GetForUpdate(gomock.Any(), int32(8), int32(1)).Return(domain.Address{}, domain.ErrAddressNotFound)
	require.ErrorIs(t, e.Delete(context.Background(), 8, 1), domain.ErrAddressNotFound)
}

func TestAddressUsecase_UpdateDefault(t *testing.T) {
	e := newAddressTestEnv(t)
	a := domain.Address{ID: 2, UserID: 7, Street: "street"}
	lock := expectAddressLock(e, 7)
	updated := a
	updated.IsDefault = true
	gomock.InOrder(
		lock,
		e.repo.EXPECT().GetForUpdate(gomock.Any(), int32(7), int32(2)).Return(a, nil),
		e.repo.EXPECT().ClearDefault(gomock.Any(), int32(7)).Return(nil),
		e.repo.EXPECT().Update(gomock.Any(), updated).Return(updated, nil),
	)
	got, err := e.Update(context.Background(), 7, 2, domain.AddressInput{IsDefault: flag(true)})
	require.NoError(t, err)
	require.Equal(t, updated, got)
	lock = expectAddressLock(e, 7)
	a = updated
	updated.IsDefault = false
	updated.Comment = "intercom 34"
	gomock.InOrder(
		lock,
		e.repo.EXPECT().GetForUpdate(gomock.Any(), int32(7), int32(2)).Return(a, nil),
		e.repo.EXPECT().Count(gomock.Any(), int32(7)).Return(2, nil),
		e.repo.EXPECT().Update(gomock.Any(), updated).Return(updated, nil),
		e.repo.EXPECT().PromoteOldest(gomock.Any(), int32(7), int32(2)).Return(nil),
	)
	got, err = e.Update(context.Background(), 7, 2, domain.AddressInput{IsDefault: flag(false), Comment: str(" intercom 34 ")})
	require.NoError(t, err)
	require.Equal(t, updated, got)
}

func TestAddressUsecase_OnlyAddressStaysDefault(t *testing.T) {
	e := newAddressTestEnv(t)
	expectAddressLock(e, 7)
	a := domain.Address{ID: 1, UserID: 7, IsDefault: true}
	e.repo.EXPECT().GetForUpdate(gomock.Any(), int32(7), int32(1)).Return(a, nil)
	e.repo.EXPECT().Count(gomock.Any(), int32(7)).Return(1, nil)
	e.repo.EXPECT().Update(gomock.Any(), a).Return(a, nil)
	got, err := e.Update(context.Background(), 7, 1, domain.AddressInput{IsDefault: flag(false)})
	require.NoError(t, err)
	require.True(t, got.IsDefault)
}

func TestAddressUsecase_Validation(t *testing.T) {
	tests := []struct {
		name string
		in   domain.AddressInput
		want error
	}{
		{"no city", domain.AddressInput{Street: str("Тверская"), House: str("д. 1")}, domain.ErrMissingCity},
		{"blank city", domain.AddressInput{City: str(" "), Street: str("Тверская"), House: str("д. 1")}, domain.ErrMissingCity},
		{"long city", domain.AddressInput{City: str(strings.Repeat("я", maxCityLength+1)), Street: str("Тверская"), House: str("д. 1")},
			domain.ErrCityTooLong},
		{"no street", domain.AddressInput{City: str("Москва"), House: str("д. 1")}, domain.ErrMissingStreet},
		{"blank street", domain.AddressInput{City: str("Москва"), Street: str("  "), House: str("д. 1")}, domain.ErrMissingStreet},
		{"long street", domain.AddressInput{City: str("Москва"), Street: str(strings.Repeat("я", maxStreetLength+1)), House: str("д. 1")},
			domain.ErrStreetTooLong},
		{"no house", domain.AddressInput{City: str("Москва"), Street: str("Тверская")}, domain.ErrMissingHouse},
		{"long house", domain.AddressInput{City: str("Москва"), Street: str("Тверская"), House: str(strings.Repeat("я", maxHouseLength+1))},
			domain.ErrHouseTooLong},
		{"long apartment", domain.AddressInput{City: str("Москва"), Street: str("Тверская"), House: str("д. 1"),
			Apartment: str(strings.Repeat("1", maxApartmentLength+1))}, domain.ErrApartmentTooLong},
		{"long comment", domain.AddressInput{City: str("Москва"), Street: str("Тверская"), House: str("д. 1"),
			Comment: str(strings.Repeat("я", maxAddressCommentLength+1))}, domain.ErrAddressCommentTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := newAddressTestEnv(t)
			_, err := uc.Create(context.Background(), 7, tt.in)
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestAddressUsecase_UpdateValidatesOnlyPassedFields(t *testing.T) {
	e := newAddressTestEnv(t)
	_, err := e.Update(context.Background(), 7, 1, domain.AddressInput{House: str(" ")})
	require.ErrorIs(t, err, domain.ErrMissingHouse)
	expectAddressLock(e, 7)
	e.repo.EXPECT().GetForUpdate(gomock.Any(), int32(7), int32(2)).Return(domain.Address{}, domain.ErrAddressNotFound)
	_, err = e.Update(context.Background(), 7, 2, domain.AddressInput{})
	require.ErrorIs(t, err, domain.ErrAddressNotFound)
}
