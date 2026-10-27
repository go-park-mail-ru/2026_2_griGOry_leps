//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type sellerFixture struct {
	pool   *pgxpool.Pool
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

	_, err = pool.Exec(ctx, `INSERT INTO category (category_id, parent_id, name, slug) OVERRIDING SYSTEM VALUE VALUES
		(1, NULL, 'Спорт и отдых', 'sport'), (2, 1, 'Велосипеды', 'bikes'), (3, NULL, 'Книги', 'books')`)
	require.NoError(t, err)

	return sellerFixture{pool: pool, seller: seller.ID, buyer: buyer.ID}
}

func (f sellerFixture) ad(t *testing.T, categoryID int32, title string, price int64, status string, age time.Duration) int32 {
	t.Helper()
	var id int32
	err := f.pool.QueryRow(context.Background(), `INSERT INTO ad
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

func TestAdRepo_ListByUser(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	repo := NewAdRepository(f.pool)

	old := f.ad(t, 2, "Велосипед", 100, domain.AdStatusActive, 3*time.Hour)
	fresh := f.ad(t, 3, "Книга", 100, domain.AdStatusActive, time.Hour)
	draft := f.ad(t, 3, "Черновик", 100, domain.AdStatusDraft, 2*time.Hour)
	f.ad(t, 3, "Отклонено", 100, domain.AdStatusRejected, 4*time.Hour)

	items, counts, err := repo.ListByUser(ctx, domain.MyAdFilter{UserID: f.seller, Limit: 3})
	require.NoError(t, err)
	require.Equal(t, map[string]int{"active": 2, "draft": 1, "rejected": 1}, counts)
	require.Equal(t, []int32{fresh, draft, old}, adIDs(items))

	items, _, err = repo.ListByUser(ctx, domain.MyAdFilter{UserID: f.seller, Status: domain.AdStatusActive, Limit: 10, Offset: 1})
	require.NoError(t, err)
	require.Equal(t, []int32{old}, adIDs(items))

	items, counts, err = repo.ListByUser(ctx, domain.MyAdFilter{UserID: f.buyer, Limit: 10})
	require.NoError(t, err)
	require.Empty(t, items)
	require.Empty(t, counts)
}

func TestAdRepo_ListByUser_DraftWithoutPriceAndClosed(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()

	var draft int32
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO ad (user_id, title, status) VALUES ($1, 'Без цены', 'draft') RETURNING id`, f.seller).Scan(&draft))
	closed := f.ad(t, 2, "Велосипед", 100, domain.AdStatusActive, time.Hour)
	_, err := f.pool.Exec(ctx, `UPDATE ad SET status = 'closed', close_reason = 'changed_mind' WHERE id = $1`, closed)
	require.NoError(t, err)

	items, counts, err := NewAdRepository(f.pool).ListByUser(ctx, domain.MyAdFilter{UserID: f.seller, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, map[string]int{"draft": 1, "closed": 1}, counts)
	require.Equal(t, []int32{closed, draft}, adIDs(items))
	require.Equal(t, "changed_mind", items[0].CloseReason)
	require.WithinDuration(t, time.Now(), items[0].UpdatedAt, time.Minute)
	require.Nil(t, items[1].Price)
	require.Zero(t, items[1].CategoryID)
}

func TestAdRepo_IsFreeInFeed(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	id := f.ad(t, 3, "Отдам книги", 0, domain.AdStatusActive, time.Hour)
	_, err := f.pool.Exec(ctx, `UPDATE ad SET is_free = true WHERE id = $1`, id)
	require.NoError(t, err)

	items, _, err := NewAdRepository(f.pool).List(ctx, domain.AdFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.True(t, items[0].IsFree)
	require.Equal(t, int64(0), *items[0].Price)
}
