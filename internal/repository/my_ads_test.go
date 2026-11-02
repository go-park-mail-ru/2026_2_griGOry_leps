package repository

import (
	"context"
	"testing"
	"time"

	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type sellerFixture struct {
	pool   *sql.DB
	seller int32
	buyer  int32
}

func newSellerFixture(t *testing.T) sellerFixture {
	t.Helper()
	pool := newTestPool(t)
	ctx := context.Background()
	users := NewUserRepository(pool)

	seller, err := users.Create(ctx, "seller@goget.pw", "hash", "Тимур", "+79990000001")
	require.NoError(t, err)
	buyer, err := users.Create(ctx, "buyer@goget.pw", "hash", "Анна", "+79990000002")
	require.NoError(t, err)

	_, err = pool.ExecContext(ctx, `INSERT INTO category (category_id, parent_id, name, slug) OVERRIDING SYSTEM VALUE VALUES
		(1, NULL, 'Спорт и отдых', 'sport'), (2, 1, 'Велосипеды', 'bikes'), (3, NULL, 'Книги', 'books')`)
	require.NoError(t, err)

	return sellerFixture{pool: pool, seller: seller.ID, buyer: buyer.ID}
}

func (f sellerFixture) ad(t *testing.T, categoryID int32, title string, price int64, status string, age time.Duration) int32 {
	t.Helper()
	var id int32
	err := f.pool.QueryRowContext(context.Background(), `INSERT INTO ad
		(user_id, category_id, title, price, city, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'Москва', $5, now() - $6::float8 * interval '1 second', now() - $6::float8 * interval '1 second')
		RETURNING id`, f.seller, categoryID, title, price, status, age.Seconds()).Scan(&id)
	require.NoError(t, err)
	return id
}

func adIDs(ads []domain.Ad) []int32 {
	ids := make([]int32, 0, len(ads))
	for _, ad := range ads {
		ids = append(ids, ad.ID)
	}
	return ids
}

func expectMyAds(mock sqlmock.Sqlmock, f domain.MyAdFilter, counts map[string]int, ads ...domain.Ad) {
	rows := sqlmock.NewRows([]string{"status", "count"})
	for status, count := range counts {
		rows.AddRow(status, count)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(countMyAdsQuery).WithArgs(f.UserID).WillReturnRows(rows).RowsWillBeClosed()
	mock.ExpectQuery(listMyAdsQuery).WithArgs(f.UserID, f.Status, f.Limit, f.Offset).WillReturnRows(adMockRows(ads...)).RowsWillBeClosed()
	mock.ExpectCommit()
}
func TestAdRepo_ListByUser(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewAdRepository(db)
	counts := map[string]int{"active": 2, "draft": 1, "rejected": 1}
	f := domain.MyAdFilter{UserID: 1, Limit: 3}
	expectMyAds(mock, f, counts, domain.Ad{ID: 2}, domain.Ad{ID: 3}, domain.Ad{ID: 1})
	items, actual, err := repo.ListByUser(context.Background(), f)
	require.NoError(t, err)
	require.Equal(t, counts, actual)
	require.Equal(t, []int32{2, 3, 1}, adIDs(items))
	f = domain.MyAdFilter{UserID: 1, Status: domain.AdStatusActive, Limit: 10, Offset: 1}
	expectMyAds(mock, f, counts, domain.Ad{ID: 1, Status: domain.AdStatusActive})
	items, _, err = repo.ListByUser(context.Background(), f)
	require.NoError(t, err)
	require.Equal(t, []int32{1}, adIDs(items))
	f = domain.MyAdFilter{UserID: 2, Limit: 10}
	expectMyAds(mock, f, nil)
	items, actual, err = repo.ListByUser(context.Background(), f)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Empty(t, actual)
}
func TestAdRepo_ListByUser_DraftWithoutPriceAndClosed(t *testing.T) {
	db, mock := newSQLMock(t)
	f := domain.MyAdFilter{UserID: 1, Limit: 10}
	counts := map[string]int{"draft": 1, "closed": 1}
	expectMyAds(mock, f, counts, domain.Ad{ID: 2, Status: "closed", CloseReason: "changed_mind", UpdatedAt: time.Now()}, domain.Ad{ID: 1, Status: "draft"})
	items, actual, err := NewAdRepository(db).ListByUser(context.Background(), f)
	require.NoError(t, err)
	require.Equal(t, counts, actual)
	require.Equal(t, []int32{2, 1}, adIDs(items))
	require.Equal(t, "changed_mind", items[0].CloseReason)
	require.WithinDuration(t, time.Now(), items[0].UpdatedAt, time.Minute)
	require.Nil(t, items[1].Price)
	require.Zero(t, items[1].CategoryID)
}
func TestAdRepo_IsFreeInFeed(t *testing.T) {
	db, mock := newSQLMock(t)
	f := domain.AdFilter{Limit: 10}
	expectAdList(mock, f, 1, domain.Ad{ID: 1, Price: price(0), IsFree: true, Status: domain.AdStatusActive})
	items, _, err := NewAdRepository(db).List(context.Background(), f)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.True(t, items[0].IsFree)
	require.Equal(t, int64(0), *items[0].Price)
}
