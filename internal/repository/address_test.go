//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestAddressRepo_CRUD(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	repo := NewAddressRepository(f.pool)

	first, err := repo.Create(ctx, domain.Address{UserID: f.seller, Street: "Тверская", House: "д. 12", Comment: "Домофон", IsDefault: true})
	require.NoError(t, err)
	require.NotZero(t, first.ID)
	second, err := repo.Create(ctx, domain.Address{UserID: f.seller, Street: "Ленинский", House: "д. 1"})
	require.NoError(t, err)
	_, err = repo.Create(ctx, domain.Address{UserID: f.buyer, Street: "Арбат", House: "д. 5", IsDefault: true})
	require.NoError(t, err)

	list, err := repo.List(ctx, f.seller)
	require.NoError(t, err)
	require.Equal(t, []int32{first.ID, second.ID}, []int32{list[0].ID, list[1].ID})
	count, err := repo.Count(ctx, f.seller)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	second.House = "д. 2"
	updated, err := repo.Update(ctx, second)
	require.NoError(t, err)
	require.Equal(t, "д. 2", updated.House)

	_, err = repo.GetForUpdate(ctx, f.buyer, first.ID)
	require.ErrorIs(t, err, domain.ErrAddressNotFound)
	require.ErrorIs(t, repo.Delete(ctx, f.buyer, first.ID), domain.ErrAddressNotFound)
	_, err = repo.Update(ctx, domain.Address{ID: first.ID, UserID: f.buyer, Street: "x", House: "y"})
	require.ErrorIs(t, err, domain.ErrAddressNotFound)

	require.NoError(t, repo.Delete(ctx, f.seller, second.ID))
	list, err = repo.List(ctx, f.seller)
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestAddressRepo_SingleDefault(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	repo := NewAddressRepository(f.pool)

	first, err := repo.Create(ctx, domain.Address{UserID: f.seller, Street: "Тверская", House: "д. 12", IsDefault: true})
	require.NoError(t, err)
	_, err = repo.Create(ctx, domain.Address{UserID: f.seller, Street: "Ленинский", House: "д. 1", IsDefault: true})
	require.Error(t, err)

	second, err := repo.Create(ctx, domain.Address{UserID: f.seller, Street: "Ленинский", House: "д. 1"})
	require.NoError(t, err)
	third, err := repo.Create(ctx, domain.Address{UserID: f.seller, Street: "Арбат", House: "д. 5"})
	require.NoError(t, err)

	require.NoError(t, repo.ClearDefault(ctx, f.seller))
	require.NoError(t, repo.PromoteOldest(ctx, f.seller, first.ID))
	a, err := repo.GetForUpdate(ctx, f.seller, second.ID)
	require.NoError(t, err)
	require.True(t, a.IsDefault)

	require.NoError(t, repo.ClearDefault(ctx, f.seller))
	require.NoError(t, repo.PromoteOldest(ctx, f.seller, 0))
	a, err = repo.GetForUpdate(ctx, f.seller, first.ID)
	require.NoError(t, err)
	require.True(t, a.IsDefault)
	a, err = repo.GetForUpdate(ctx, f.seller, third.ID)
	require.NoError(t, err)
	require.False(t, a.IsDefault)
}

func TestCityRepo_Search(t *testing.T) {
	repo := NewCityRepository(newTestPool(t))
	ctx := context.Background()

	cities, err := repo.Search(ctx, "мос", 10)
	require.NoError(t, err)
	require.Len(t, cities, 1)
	require.Equal(t, []domain.City{{ID: cities[0].ID, Name: "Москва"}}, cities)

	cities, err = repo.Search(ctx, "", 3)
	require.NoError(t, err)
	require.Len(t, cities, 3)
	require.Equal(t, "Москва", cities[0].Name)

	cities, err = repo.Search(ctx, "%", 10)
	require.NoError(t, err)
	require.Empty(t, cities)
}
