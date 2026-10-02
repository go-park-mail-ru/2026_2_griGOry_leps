package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func sampleAds() []domain.Ad {
	now := time.Now()

	return []domain.Ad{
		{
			ID: 1, Title: "Велосипед горный", Description: "Отличное состояние",
			Price: 1500000, CategoryID: 1, City: "Москва",
			HasDelivery: true, Status: domain.AdStatusActive,
			CreatedAt: now.Add(-3 * time.Hour),
		},
		{
			ID: 2, Title: "Самокат детский", Description: "Лёгкий",
			Price: 300000, CategoryID: 1, City: "Санкт-Петербург",
			HasDelivery: false, Status: domain.AdStatusActive,
			CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			ID: 3, Title: "Книга", Description: "Интересная",
			Price: 50000, CategoryID: 2, City: "Москва",
			HasDelivery: false, Status: domain.AdStatusActive,
			CreatedAt: now.Add(-1 * time.Hour),
		},
		{
			ID: 4, Title: "Снятое", Description: "",
			Price: 100000, CategoryID: 1, City: "Москва",
			HasDelivery: false, Status: "archived",
			CreatedAt: now,
		},
	}
}

func TestAdRepo_List_OnlyActive(t *testing.T) {
	repo := NewAdRepository(sampleAds())

	items, total, err := repo.List(context.Background(), domain.AdFilter{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, items, 3)
	for _, ad := range items {
		require.Equal(t, domain.AdStatusActive, ad.Status)
	}
}

func TestAdRepo_List_FilterByCategory(t *testing.T) {
	repo := NewAdRepository(sampleAds())

	items, total, err := repo.List(context.Background(), domain.AdFilter{CategoryID: 1, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, items, 2)
}

func TestAdRepo_List_FilterByCity(t *testing.T) {
	repo := NewAdRepository(sampleAds())

	items, total, err := repo.List(context.Background(), domain.AdFilter{City: "москва", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, items, 2)
}

func TestAdRepo_List_FilterByPrice(t *testing.T) {
	repo := NewAdRepository(sampleAds())

	min := int64(100000)
	max := int64(1000000)
	items, total, err := repo.List(context.Background(), domain.AdFilter{
		PriceMin: &min, PriceMax: &max, Limit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, int32(2), items[0].ID)
}

func TestAdRepo_List_FilterByDelivery(t *testing.T) {
	repo := NewAdRepository(sampleAds())

	withDelivery := true
	items, total, err := repo.List(context.Background(), domain.AdFilter{
		HasDelivery: &withDelivery, Limit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, int32(1), items[0].ID)

	noDelivery := false
	items, total, err = repo.List(context.Background(), domain.AdFilter{
		HasDelivery: &noDelivery, Limit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, items, 2)
}

func TestAdRepo_List_SearchQuery(t *testing.T) {
	repo := NewAdRepository(sampleAds())

	items, total, err := repo.List(context.Background(), domain.AdFilter{Query: "велосипед", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, int32(1), items[0].ID)

	items, total, err = repo.List(context.Background(), domain.AdFilter{Query: "интересная", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, int32(3), items[0].ID)
}

func TestAdRepo_List_SortPriceAsc(t *testing.T) {
	repo := NewAdRepository(sampleAds())

	items, _, err := repo.List(context.Background(), domain.AdFilter{
		Sort: domain.AdSortPriceAsc, Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, int32(3), items[0].ID)
	require.Equal(t, int32(1), items[2].ID)
}

func TestAdRepo_List_SortPriceDesc(t *testing.T) {
	repo := NewAdRepository(sampleAds())

	items, _, err := repo.List(context.Background(), domain.AdFilter{
		Sort: domain.AdSortPriceDesc, Limit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), items[0].ID)
	require.Equal(t, int32(3), items[2].ID)
}

func TestAdRepo_List_SortNewest(t *testing.T) {
	repo := NewAdRepository(sampleAds())

	items, _, err := repo.List(context.Background(), domain.AdFilter{
		Sort: domain.AdSortNewest, Limit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, int32(3), items[0].ID)
	require.Equal(t, int32(2), items[1].ID)
	require.Equal(t, int32(1), items[2].ID)
}

func TestAdRepo_List_Pagination(t *testing.T) {
	repo := NewAdRepository(sampleAds())

	items, total, err := repo.List(context.Background(), domain.AdFilter{Limit: 2, Offset: 0})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, items, 2)

	items, total, err = repo.List(context.Background(), domain.AdFilter{Limit: 2, Offset: 2})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, items, 1)
}

func TestAdRepo_List_OffsetBeyondTotal(t *testing.T) {
	repo := NewAdRepository(sampleAds())

	items, total, err := repo.List(context.Background(), domain.AdFilter{Limit: 10, Offset: 100})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Empty(t, items)
}
