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

func (f sellerFixture) review(t *testing.T, authorID, adID int32, rating float64, age time.Duration, photos ...string) int32 {
	t.Helper()
	ctx := context.Background()
	var id int32
	err := f.pool.QueryRow(ctx, `INSERT INTO review (user_id, ad_id, text, rating, created_at)
		VALUES ($1, $2, 'Отзыв', $3, now() - $4::float8 * interval '1 second') RETURNING rew_id`,
		authorID, adID, rating, age.Seconds()).Scan(&id)
	require.NoError(t, err)
	for i, url := range photos {
		_, err := f.pool.Exec(ctx, `INSERT INTO review_image (rew_id, url, position) VALUES ($1, $2, $3)`, id, url, len(photos)-i)
		require.NoError(t, err)
	}
	return id
}

func (f sellerFixture) author(t *testing.T, email, phone string) int32 {
	t.Helper()
	user, err := NewUserRepository(f.pool).Create(context.Background(), email, "hash", "Олег", phone)
	require.NoError(t, err)
	return user.ID
}

func TestSellerRepo_Get(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	_, err := f.pool.Exec(ctx, `UPDATE user_account
		SET phone_verified = true, city_id = (SELECT city_id FROM city WHERE name = 'Москва') WHERE user_id = $1`, f.seller)
	require.NoError(t, err)

	bike := f.ad(t, 2, "Велосипед", 100, domain.AdStatusActive, time.Hour)
	f.ad(t, 3, "Книга", 100, domain.AdStatusActive, time.Hour)
	sold := f.ad(t, 3, "Учебник", 100, domain.AdStatusSold, time.Hour)
	f.ad(t, 3, "Черновик", 100, domain.AdStatusDraft, time.Hour)
	f.review(t, f.buyer, bike, 5, time.Hour)
	f.review(t, f.buyer, sold, 4, time.Hour)

	s, err := NewSellerRepository(f.pool).Get(ctx, f.seller)
	require.NoError(t, err)
	require.Equal(t, "Тимур", s.FirstName)
	require.Equal(t, "Москва", s.Location)
	require.True(t, s.PhoneVerified)
	require.InDelta(t, 4.5, s.Rating, 0.001)
	require.Equal(t, 2, s.ReviewsCount)
	require.Equal(t, 1, s.CompletedSalesCount)
	require.Equal(t, 2, s.ActiveListingsCount)

	_, err = NewSellerRepository(f.pool).Get(ctx, 999)
	require.ErrorIs(t, err, domain.ErrSellerNotFound)
}

func TestSellerRepo_ListAds(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	repo := NewSellerRepository(f.pool)

	f.ad(t, 2, "Велосипед дорогой", 9000, domain.AdStatusActive, 3*time.Hour)
	cheap := f.ad(t, 2, "Велосипед дешёвый", 1000, domain.AdStatusActive, 2*time.Hour)
	book := f.ad(t, 3, "Книга", 5000, domain.AdStatusActive, time.Hour)
	f.ad(t, 3, "Снятая книга", 10, domain.AdStatusClosed, time.Minute)

	page, err := repo.ListAds(ctx, domain.SellerAdFilter{SellerID: f.seller, Sort: domain.AdSortNewest, Limit: 24})
	require.NoError(t, err)
	require.Equal(t, 3, page.Total)
	require.Equal(t, book, page.Items[0].ID)
	require.Equal(t, []domain.SellerCategory{{ID: 2, Name: "Велосипеды", Count: 2}, {ID: 3, Name: "Книги", Count: 1}}, page.Categories)

	page, err = repo.ListAds(ctx, domain.SellerAdFilter{SellerID: f.seller, CategoryID: 2, Sort: domain.AdSortPriceAsc, Limit: 1})
	require.NoError(t, err)
	require.Equal(t, 2, page.Total)
	require.Len(t, page.Items, 1)
	require.Equal(t, cheap, page.Items[0].ID)
	require.Len(t, page.Categories, 2)

	page, err = repo.ListAds(ctx, domain.SellerAdFilter{SellerID: f.buyer, Sort: domain.AdSortNewest, Limit: 24})
	require.NoError(t, err)
	require.Zero(t, page.Total)
	require.Empty(t, page.Items)
	require.Empty(t, page.Categories)

	_, err = repo.ListAds(ctx, domain.SellerAdFilter{SellerID: 999, Sort: domain.AdSortNewest, Limit: 24})
	require.ErrorIs(t, err, domain.ErrSellerNotFound)
}

func TestSellerRepo_ListReviews(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	repo := NewSellerRepository(f.pool)

	first := f.ad(t, 2, "Велосипед", 100, domain.AdStatusActive, time.Hour)
	second := f.ad(t, 3, "Книга", 100, domain.AdStatusSold, time.Hour)
	other := f.author(t, "oleg@goget.pw", "+79990000003")

	best := f.review(t, f.buyer, first, 5, 3*time.Hour)
	middle := f.review(t, f.buyer, second, 3.6, 2*time.Hour, "/uploads/reviews/b.jpg", "/uploads/reviews/a.jpg")
	worst := f.review(t, other, first, 1, time.Hour)

	page, err := repo.ListReviews(ctx, domain.ReviewFilter{SellerID: f.seller, Sort: domain.ReviewSortNewest, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 3, page.Total)
	require.Equal(t, []int32{worst, middle, best}, reviewIDs(page.Items))
	require.InDelta(t, 3.2, page.Rating, 0.001)
	require.Equal(t, map[int]int{1: 1, 2: 0, 3: 0, 4: 1, 5: 1}, page.Distribution)
	require.Equal(t, "Олег", page.Items[0].Author.FirstName)
	require.Equal(t, "Велосипед", page.Items[0].ListingTitle)
	require.Empty(t, page.Items[0].Photos)
	require.Equal(t, []string{"/uploads/reviews/a.jpg", "/uploads/reviews/b.jpg"}, page.Items[1].Photos)

	page, err = repo.ListReviews(ctx, domain.ReviewFilter{SellerID: f.seller, Sort: domain.ReviewSortRatingAsc, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, []int32{worst, middle, best}, reviewIDs(page.Items))

	page, err = repo.ListReviews(ctx, domain.ReviewFilter{SellerID: f.seller, Sort: domain.ReviewSortRatingDesc, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, []int32{best, middle, worst}, reviewIDs(page.Items))

	page, err = repo.ListReviews(ctx, domain.ReviewFilter{SellerID: f.seller, Sort: domain.ReviewSortNewest, WithPhotos: true, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Equal(t, []int32{middle}, reviewIDs(page.Items))
	require.Equal(t, 1, page.Distribution[5])

	_, err = repo.ListReviews(ctx, domain.ReviewFilter{SellerID: 999, Sort: domain.ReviewSortNewest, Limit: 10})
	require.ErrorIs(t, err, domain.ErrSellerNotFound)
}

func TestSellerRepo_ListReviews_Empty(t *testing.T) {
	f := newSellerFixture(t)

	page, err := NewSellerRepository(f.pool).ListReviews(context.Background(),
		domain.ReviewFilter{SellerID: f.seller, Sort: domain.ReviewSortNewest, Limit: 10})
	require.NoError(t, err)
	require.Zero(t, page.Total)
	require.Empty(t, page.Items)
	require.Zero(t, page.Rating)
	require.Equal(t, map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0}, page.Distribution)
}

func reviewIDs(reviews []domain.Review) []int32 {
	ids := make([]int32, 0, len(reviews))
	for _, rv := range reviews {
		ids = append(ids, rv.ID)
	}
	return ids
}

func TestSellerRepo_CreateReport(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	repo := NewSellerRepository(f.pool)

	err := repo.CreateReport(ctx, domain.SellerReport{
		SellerID: f.seller, ReporterID: f.buyer, Reason: domain.ReportReasonRude, Comment: "Хамил",
	})
	require.NoError(t, err)

	var reason, comment string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT reason, comment FROM seller_report WHERE seller_id = $1`, f.seller).
		Scan(&reason, &comment))
	require.Equal(t, "rude", reason)
	require.Equal(t, "Хамил", comment)

	err = repo.CreateReport(ctx, domain.SellerReport{SellerID: 999, ReporterID: f.buyer, Reason: domain.ReportReasonFraud})
	require.ErrorIs(t, err, domain.ErrSellerNotFound)
}
