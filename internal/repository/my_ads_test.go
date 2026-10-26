//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

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

func adIDs(ads []domain.Ad) []int32 {
	ids := make([]int32, 0, len(ads))
	for _, ad := range ads {
		ids = append(ids, ad.ID)
	}
	return ids
}

func TestAdRepo_CloseAndDelete(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	repo := NewAdRepository(f.pool)
	tx := NewTransactor(f.pool)
	id := f.ad(t, 2, "Велосипед", 100, domain.AdStatusActive, time.Hour)

	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		ownerID, status, err := repo.GetForUpdate(ctx, id)
		require.NoError(t, err)
		require.Equal(t, f.seller, ownerID)
		require.Equal(t, domain.AdStatusActive, status)
		return repo.Close(ctx, id, domain.CloseReasonChangedMind)
	})
	require.NoError(t, err)

	items, counts, err := repo.ListByUser(ctx, domain.MyAdFilter{UserID: f.seller, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, map[string]int{"closed": 1}, counts)
	require.Equal(t, domain.CloseReasonChangedMind, items[0].CloseReason)
	require.WithinDuration(t, time.Now(), items[0].UpdatedAt, time.Minute)

	require.NoError(t, repo.Delete(ctx, id))
	_, _, err = repo.GetForUpdate(ctx, id)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
	require.ErrorIs(t, repo.Delete(ctx, id), domain.ErrListingNotFound)
	require.ErrorIs(t, repo.Close(ctx, id, domain.CloseReasonOther), domain.ErrListingNotFound)
}

func TestAdRepo_GetListing(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	repo := NewAdRepository(f.pool)
	id := f.ad(t, 2, "Велосипед", 1500000, domain.AdStatusActive, time.Hour)
	f.review(t, f.buyer, id, 4, time.Hour)

	_, err := f.pool.Exec(ctx, `INSERT INTO ad_image (ad_id, url, position) VALUES ($1, '/b.jpg', 1), ($1, '/a.jpg', 0)`, id)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `WITH c AS (INSERT INTO cart (user_id) VALUES ($1) RETURNING id)
		INSERT INTO cart_item (cart_id, ad_id) SELECT id, $2 FROM c`, f.buyer, id)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `UPDATE ad SET status = 'closed', close_reason = 'sold_elsewhere' WHERE id = $1`, id)
	require.NoError(t, err)

	l, err := repo.GetListing(ctx, id, f.seller)
	require.NoError(t, err)
	require.True(t, l.IsOwner)
	require.False(t, l.InCart)
	require.Equal(t, domain.CloseReasonSoldElsewhere, l.CloseReason)
	require.Equal(t, &domain.ListingCategory{ID: 2, Name: "Велосипеды", Path: "Спорт и отдых › Велосипеды"}, l.Category)
	require.Equal(t, []string{"/a.jpg", "/b.jpg"}, []string{l.Photos[0].URL, l.Photos[1].URL})
	require.Equal(t, "/a.jpg", l.ImageURL)
	require.Equal(t, f.seller, l.Seller.ID)
	require.Equal(t, "Тимур", l.Seller.FirstName)
	require.InDelta(t, 4, l.Seller.Rating, 0.001)
	require.Equal(t, 1, l.Seller.ReviewsCount)
	require.NotNil(t, l.PublishedAt)

	l, err = repo.GetListing(ctx, id, f.buyer)
	require.NoError(t, err)
	require.False(t, l.IsOwner)
	require.True(t, l.InCart)
	require.Empty(t, l.CloseReason)

	_, err = repo.GetListing(ctx, 999, f.seller)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
}

func TestAdRepo_GetListing_Draft(t *testing.T) {
	f := newSellerFixture(t)
	var id int32
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`INSERT INTO ad (user_id, title, price) VALUES ($1, 'Черновик', 0) RETURNING id`, f.seller).Scan(&id))

	l, err := NewAdRepository(f.pool).GetListing(context.Background(), id, f.seller)
	require.NoError(t, err)
	require.Nil(t, l.Category)
	require.Empty(t, l.Photos)
	require.Nil(t, l.PublishedAt)
	require.Equal(t, domain.AdStatusDraft, l.Status)
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
}
