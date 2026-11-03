package repository

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func price(kopecks int64) *int64 { return &kopecks }

func sampleAds() []domain.Ad {
	now := time.Now()

	return []domain.Ad{
		{
			ID: 1, Title: "Велосипед горный", Description: "Отличное состояние",
			Price: price(1500000), CategoryID: 1, City: "Москва",
			HasDelivery: true, Status: domain.AdStatusActive,
			CreatedAt: now.Add(-3 * time.Hour),
		},
		{
			ID: 2, Title: "Самокат детский", Description: "Лёгкий",
			Price: price(300000), CategoryID: 1, City: "Санкт-Петербург",
			HasDelivery: false, Status: domain.AdStatusActive,
			CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			ID: 3, Title: "Книга", Description: "Интересная",
			Price: price(50000), CategoryID: 2, City: "Москва",
			HasDelivery: false, Status: domain.AdStatusActive,
			CreatedAt: now.Add(-1 * time.Hour),
		},
		{
			ID: 4, Title: "Снятое", Description: "",
			Price: price(100000), CategoryID: 1, City: "Москва",
			HasDelivery: false, Status: "closed",
			CreatedAt: now,
		},
	}
}

func TestAdRepo_List_OnlyActive(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	ads := sampleAds()
	expectAdList(mock, domain.AdFilter{Limit: 10}, 3, ads[2], ads[1], ads[0])

	items, total, err := repo.List(context.Background(), domain.AdFilter{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, items, 3)
	for _, ad := range items {
		require.Equal(t, domain.AdStatusActive, ad.Status)
	}
}

func TestAdRepo_List_FilterByCategory(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	ads := sampleAds()
	expectAdList(mock, domain.AdFilter{CategoryID: 1, Limit: 10}, 2, ads[1], ads[0])

	items, total, err := repo.List(context.Background(), domain.AdFilter{CategoryID: 1, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, items, 2)
}

func TestAdRepo_List_FilterByCity(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	ads := sampleAds()
	expectAdList(mock, domain.AdFilter{City: "москва", Limit: 10}, 2, ads[2], ads[0])

	items, total, err := repo.List(context.Background(), domain.AdFilter{City: "москва", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, items, 2)
}

func TestAdRepo_List_FilterByPrice(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	ads := sampleAds()
	expectAdList(mock, domain.AdFilter{PriceMin: price(100000), PriceMax: price(1000000), Limit: 10}, 1, ads[1])

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
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	ads := sampleAds()
	yes, no := true, false
	expectAdList(mock, domain.AdFilter{HasDelivery: &yes, Limit: 10}, 1, ads[0])
	expectAdList(mock, domain.AdFilter{HasDelivery: &no, Limit: 10}, 2, ads[2], ads[1])

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
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	ads := sampleAds()
	expectAdList(mock, domain.AdFilter{Query: "велосипед", Limit: 10}, 1, ads[0])
	expectAdList(mock, domain.AdFilter{Query: "интересная", Limit: 10}, 1, ads[2])

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
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	ads := sampleAds()
	expectAdList(mock, domain.AdFilter{Sort: domain.AdSortPriceAsc, Limit: 10}, 3, ads[2], ads[1], ads[0])

	items, _, err := repo.List(context.Background(), domain.AdFilter{
		Sort: domain.AdSortPriceAsc, Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, int32(3), items[0].ID)
	require.Equal(t, int32(1), items[2].ID)
}

func TestAdRepo_List_SortPriceDesc(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	ads := sampleAds()
	expectAdList(mock, domain.AdFilter{Sort: domain.AdSortPriceDesc, Limit: 10}, 3, ads[0], ads[1], ads[2])

	items, _, err := repo.List(context.Background(), domain.AdFilter{
		Sort: domain.AdSortPriceDesc, Limit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), items[0].ID)
	require.Equal(t, int32(3), items[2].ID)
}

func TestAdRepo_List_SortNewest(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	ads := sampleAds()
	expectAdList(mock, domain.AdFilter{Sort: domain.AdSortNewest, Limit: 10}, 3, ads[2], ads[1], ads[0])

	items, _, err := repo.List(context.Background(), domain.AdFilter{
		Sort: domain.AdSortNewest, Limit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, int32(3), items[0].ID)
	require.Equal(t, int32(2), items[1].ID)
	require.Equal(t, int32(1), items[2].ID)
}

func TestAdRepo_List_Pagination(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	ads := sampleAds()
	expectAdList(mock, domain.AdFilter{Limit: 2}, 3, ads[2], ads[1])
	expectAdList(mock, domain.AdFilter{Limit: 2, Offset: 2}, 3, ads[0])

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
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	expectAdList(mock, domain.AdFilter{Limit: 10, Offset: 100}, 3)

	items, total, err := repo.List(context.Background(), domain.AdFilter{Limit: 10, Offset: 100})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Empty(t, items)
}

func adMockRows(ads ...domain.Ad) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "user", "category", "title", "description", "price", "city", "delivery", "free", "image", "status", "reason", "created", "updated"})
	for _, ad := range ads {
		rows.AddRow(ad.ID, ad.UserID, ad.CategoryID, ad.Title, ad.Description, ad.Price, ad.City, ad.HasDelivery, ad.IsFree, ad.ImageURL, ad.Status, ad.CloseReason, ad.CreatedAt, ad.UpdatedAt)
	}
	return rows
}

func expectAdList(mock sqlmock.Sqlmock, f domain.AdFilter, total int, ads ...domain.Ad) {
	sort := f.Sort
	if sort == "" {
		sort = domain.AdSortNewest
	}
	args := []driver.Value{f.Query, f.CategoryID, f.City, f.PriceMin, f.PriceMax, f.HasDelivery}
	mock.ExpectBegin()
	mock.ExpectQuery(countAdsQuery).WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(total))
	mock.ExpectQuery(listAdsQuery).WithArgs(append(args, sort, f.Limit, f.Offset)...).WillReturnRows(adMockRows(ads...)).RowsWillBeClosed()
	mock.ExpectCommit()
}
