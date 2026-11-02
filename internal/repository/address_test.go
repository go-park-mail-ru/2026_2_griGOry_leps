package repository

import (
	"context"
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func addressMockRows(addresses ...domain.Address) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "user_id", "street", "house", "comment", "is_default", "created_at"})
	for _, a := range addresses {
		rows.AddRow(a.ID, a.UserID, a.Street, a.House, a.Comment, a.IsDefault, a.CreatedAt)
	}
	return rows
}
func TestAddressRepo_CRUD(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewAddressRepository(db)
	ctx := context.Background()
	first := domain.Address{ID: 1, UserID: 1, Street: "Tverskaya", House: "12", Comment: "Intercom", IsDefault: true, CreatedAt: time.Now()}
	second := domain.Address{ID: 2, UserID: 1, Street: "Leninsky", House: "1", CreatedAt: time.Now()}
	third := domain.Address{ID: 3, UserID: 2, Street: "Arbat", House: "5", IsDefault: true, CreatedAt: time.Now()}
	for _, a := range []domain.Address{first, second, third} {
		mock.ExpectQuery(createAddressQuery).WithArgs(a.UserID, a.Street, a.House, a.Comment, a.IsDefault).WillReturnRows(addressMockRows(a))
		actual, err := repo.Create(ctx, a)
		require.NoError(t, err)
		require.Equal(t, a, actual)
	}
	mock.ExpectQuery(listAddressesQuery).WithArgs(int32(1)).WillReturnRows(addressMockRows(first, second)).RowsWillBeClosed()
	list, err := repo.List(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, []domain.Address{first, second}, list)
	mock.ExpectQuery(countAddressesQuery).WithArgs(int32(1)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	count, err := repo.Count(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	second.House = "2"
	mock.ExpectQuery(updateAddressQuery).WithArgs(second.ID, second.UserID, second.Street, second.House, second.Comment, second.IsDefault).WillReturnRows(addressMockRows(second))
	updated, err := repo.Update(ctx, second)
	require.NoError(t, err)
	require.Equal(t, "2", updated.House)
	mock.ExpectQuery(getAddressForUpdateQuery).WithArgs(int32(1), int32(2)).WillReturnError(sql.ErrNoRows)
	_, err = repo.GetForUpdate(ctx, 2, 1)
	require.ErrorIs(t, err, domain.ErrAddressNotFound)
	mock.ExpectExec(deleteAddressQuery).WithArgs(int32(1), int32(2)).WillReturnResult(sqlmock.NewResult(0, 0))
	require.ErrorIs(t, repo.Delete(ctx, 2, 1), domain.ErrAddressNotFound)
	mock.ExpectQuery(updateAddressQuery).WithArgs(int32(1), int32(2), "x", "y", "", false).WillReturnError(sql.ErrNoRows)
	_, err = repo.Update(ctx, domain.Address{ID: 1, UserID: 2, Street: "x", House: "y"})
	require.ErrorIs(t, err, domain.ErrAddressNotFound)
	mock.ExpectExec(deleteAddressQuery).WithArgs(int32(2), int32(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Delete(ctx, 1, 2))
	mock.ExpectQuery(listAddressesQuery).WithArgs(int32(1)).WillReturnRows(addressMockRows(first)).RowsWillBeClosed()
	list, err = repo.List(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, []domain.Address{first}, list)
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
	db, mock := newSQLMock(t)
	repo := NewCityRepository(db)
	ctx := context.Background()
	for _, tc := range []struct {
		prefix string
		limit  int
		cities []domain.City
	}{
		{"mos", 10, []domain.City{{ID: 1, Name: "Moscow"}}},
		{"", 3, []domain.City{{ID: 1, Name: "Moscow"}, {ID: 2, Name: "Kazan"}, {ID: 3, Name: "Omsk"}}},
		{"%", 10, []domain.City{}},
	} {
		rows := sqlmock.NewRows([]string{"id", "name"})
		for _, city := range tc.cities {
			rows.AddRow(city.ID, city.Name)
		}
		mock.ExpectQuery(searchCitiesQuery).WithArgs(tc.prefix, tc.limit).WillReturnRows(rows).RowsWillBeClosed()
		cities, err := repo.Search(ctx, tc.prefix, tc.limit)
		require.NoError(t, err)
		require.Equal(t, tc.cities, cities)
	}
}
