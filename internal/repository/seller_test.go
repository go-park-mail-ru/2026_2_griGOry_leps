package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func (f sellerFixture) review(t *testing.T, authorID, adID int32, rating float64, age time.Duration) int32 {
	t.Helper()
	var id int32
	err := f.pool.QueryRowContext(context.Background(), `INSERT INTO review (user_id, ad_id, text, rating, created_at)
		VALUES ($1, $2, 'Отзыв', $3, now() - $4::float8 * interval '1 second') RETURNING rew_id`,
		authorID, adID, rating, age.Seconds()).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestSellerRepo_Get(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewSellerRepository(db)
	mock.ExpectQuery(getSellerQuery).WithArgs(int32(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "avatar", "rating", "reviews", "created", "location", "verified", "completed", "active"}).AddRow(1, "Timur", "", 4.5, 2, time.Now(), "Moscow", true, 1, 2))
	seller, err := repo.Get(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "Timur", seller.FirstName)
	require.Equal(t, "Moscow", seller.Location)
	require.True(t, seller.PhoneVerified)
	require.InDelta(t, 4.5, seller.Rating, 0.001)
	require.Equal(t, 2, seller.ReviewsCount)
	require.Equal(t, 1, seller.CompletedSalesCount)
	require.Equal(t, 2, seller.ActiveListingsCount)
	mock.ExpectQuery(getSellerQuery).WithArgs(int32(999)).WillReturnError(sql.ErrNoRows)
	_, err = repo.Get(context.Background(), 999)
	require.ErrorIs(t, err, domain.ErrSellerNotFound)
}

func TestSellerRepo_ListAds(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewSellerRepository(db)
	categories := []domain.SellerCategory{{ID: 2, Name: "Bikes", Count: 2}, {ID: 3, Name: "Books", Count: 1}}
	for _, tc := range []struct {
		filter     domain.SellerAdFilter
		total      int
		ads        []domain.Ad
		categories []domain.SellerCategory
	}{
		{domain.SellerAdFilter{SellerID: 1, Sort: domain.AdSortNewest, Limit: 24}, 3, []domain.Ad{{ID: 3}, {ID: 2}, {ID: 1}}, categories},
		{domain.SellerAdFilter{SellerID: 1, CategoryID: 2, Sort: domain.AdSortPriceAsc, Limit: 1}, 2, []domain.Ad{{ID: 2}}, categories},
		{domain.SellerAdFilter{SellerID: 2, Sort: domain.AdSortNewest, Limit: 24}, 0, []domain.Ad{}, []domain.SellerCategory{}},
	} {
		f := tc.filter
		mock.ExpectBegin()
		mock.ExpectQuery(sellerExistsQuery).WithArgs(f.SellerID).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery(countSellerAdsQuery).WithArgs(f.SellerID, f.CategoryID).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.total))
		mock.ExpectQuery(listSellerAdsQuery).WithArgs(f.SellerID, f.CategoryID, f.Sort, f.Limit, f.Offset).WillReturnRows(adMockRows(tc.ads...)).RowsWillBeClosed()
		rows := sqlmock.NewRows([]string{"id", "name", "count"})
		for _, c := range tc.categories {
			rows.AddRow(c.ID, c.Name, c.Count)
		}
		mock.ExpectQuery(listSellerCategoriesQuery).WithArgs(f.SellerID).WillReturnRows(rows).RowsWillBeClosed()
		mock.ExpectCommit()
		page, err := repo.ListAds(context.Background(), f)
		require.NoError(t, err)
		require.Equal(t, tc.total, page.Total)
		require.Equal(t, adIDs(tc.ads), adIDs(page.Items))
		require.Equal(t, tc.categories, page.Categories)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(sellerExistsQuery).WithArgs(int32(999)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectRollback()
	_, err := repo.ListAds(context.Background(), domain.SellerAdFilter{SellerID: 999, Sort: domain.AdSortNewest, Limit: 24})
	require.ErrorIs(t, err, domain.ErrSellerNotFound)
}

func TestSellerRepo_ListReviews(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewSellerRepository(db)
	distribution := map[int]int{1: 1, 2: 0, 3: 0, 4: 1, 5: 1}
	for _, tc := range []struct {
		sort   string
		photos bool
		ids    []int32
	}{
		{domain.ReviewSortNewest, false, []int32{3, 2, 1}},
		{domain.ReviewSortRatingAsc, false, []int32{3, 2, 1}},
		{domain.ReviewSortRatingDesc, false, []int32{1, 2, 3}},
		{domain.ReviewSortNewest, true, []int32{2}},
	} {
		f := domain.ReviewFilter{SellerID: 1, Sort: tc.sort, WithPhotos: tc.photos, Limit: 10}
		rows := sqlmock.NewRows([]string{"id", "author", "name", "avatar", "rating", "text", "title", "created", "photos"})
		for _, id := range tc.ids {
			photos := "{}"
			rating := map[int32]float64{1: 5, 2: 3.6, 3: 1}[id]
			if id == 2 {
				photos = "{/uploads/reviews/a.jpg,/uploads/reviews/b.jpg}"
			}
			rows.AddRow(id, 2, "Oleg", "", rating, "Review", "Bike", time.Now(), photos)
		}
		expectReviews(mock, f, len(tc.ids), rows, 3.2, distribution)
		page, err := repo.ListReviews(context.Background(), f)
		require.NoError(t, err)
		require.Equal(t, len(tc.ids), page.Total)
		require.Equal(t, tc.ids, reviewIDs(page.Items))
		require.InDelta(t, 3.2, page.Rating, 0.001)
		require.Equal(t, distribution, page.Distribution)
		for _, review := range page.Items {
			require.Equal(t, "Oleg", review.Author.FirstName)
			require.Equal(t, "Bike", review.ListingTitle)
			if review.ID == 2 {
				require.Equal(t, []string{"/uploads/reviews/a.jpg", "/uploads/reviews/b.jpg"}, review.Photos)
			} else {
				require.Empty(t, review.Photos)
			}
		}
	}
	mock.ExpectBegin()
	mock.ExpectQuery(sellerExistsQuery).WithArgs(int32(999)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectRollback()
	_, err := repo.ListReviews(context.Background(), domain.ReviewFilter{SellerID: 999, Sort: domain.ReviewSortNewest, Limit: 10})
	require.ErrorIs(t, err, domain.ErrSellerNotFound)
}

func TestSellerRepo_ListReviews_Empty(t *testing.T) {
	db, mock := newSQLMock(t)
	f := domain.ReviewFilter{SellerID: 1, Sort: domain.ReviewSortNewest, Limit: 10}
	expectReviews(mock, f, 0, sqlmock.NewRows([]string{"id", "author", "name", "avatar", "rating", "text", "title", "created", "photos"}), 0, nil)
	page, err := NewSellerRepository(db).ListReviews(context.Background(), f)
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
	db, mock := newSQLMock(t)
	repo := NewSellerRepository(db)
	report := domain.SellerReport{SellerID: 1, ReporterID: 2, Reason: domain.ReportReasonRude, Comment: "Rude"}
	mock.ExpectExec(createSellerReportQuery).WithArgs(report.SellerID, report.ReporterID, report.Reason, report.Comment).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.CreateReport(context.Background(), report))
	mock.ExpectExec(createSellerReportQuery).WithArgs(int32(999), int32(2), domain.ReportReasonFraud, "").WillReturnError(&pgconn.PgError{Code: "23503", ConstraintName: "seller_report_seller_id_fkey"})
	require.ErrorIs(t, repo.CreateReport(context.Background(), domain.SellerReport{SellerID: 999, ReporterID: 2, Reason: domain.ReportReasonFraud}), domain.ErrSellerNotFound)
}

func TestSellerRepo_ReviewsSurviveListingDeletion(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	repo := NewSellerRepository(f.pool)
	bike := f.ad(t, 2, "Велосипед", 100, domain.AdStatusActive, time.Hour)
	book := f.ad(t, 3, "Книга", 100, domain.AdStatusActive, time.Hour)
	id := f.review(t, f.buyer, bike, 1, time.Hour)

	var sellerID int32
	var title string
	require.NoError(t, f.pool.QueryRowContext(ctx, `SELECT seller_id, listing_title FROM review WHERE rew_id = $1`, id).
		Scan(&sellerID, &title))
	require.Equal(t, f.seller, sellerID)
	require.Equal(t, "Велосипед", title)

	_, err := NewListingRepository(f.pool).Delete(ctx, bike)
	require.NoError(t, err)

	page, err := repo.ListReviews(ctx, domain.ReviewFilter{SellerID: f.seller, Sort: domain.ReviewSortNewest, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, []int32{id}, reviewIDs(page.Items))
	require.Equal(t, "Велосипед", page.Items[0].ListingTitle)
	require.InDelta(t, 1, page.Rating, 0.001)
	require.Equal(t, 1, page.Distribution[1])

	s, err := repo.Get(ctx, f.seller)
	require.NoError(t, err)
	require.Equal(t, 1, s.ReviewsCount)
	require.InDelta(t, 1, s.Rating, 0.001)

	p, err := NewUserRepository(f.pool).GetProfile(ctx, f.seller)
	require.NoError(t, err)
	require.Equal(t, 1, p.ReviewsCount)

	l, err := NewListingRepository(f.pool).Get(ctx, book, f.buyer)
	require.NoError(t, err)
	require.Equal(t, 1, l.Seller.ReviewsCount)
	require.InDelta(t, 1, l.Seller.Rating, 0.001)
}

func TestSellerRepo_TriggerTakesSellerFromListing(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	bike := f.ad(t, 2, "Велосипед", 100, domain.AdStatusActive, time.Hour)

	var sellerID int32
	var title string
	err := f.pool.QueryRowContext(ctx, `INSERT INTO review (user_id, ad_id, rating, seller_id, listing_title)
		VALUES ($1, $2, 5, $1, 'Чужое') RETURNING seller_id, listing_title`, f.buyer, bike).Scan(&sellerID, &title)
	require.NoError(t, err)
	require.Equal(t, f.seller, sellerID)
	require.Equal(t, "Велосипед", title)
}

func TestSellerRepo_RatingMustBeOneToFive(t *testing.T) {
	f := newSellerFixture(t)

	for i, rating := range []float64{0, 0.5, 5.5} {
		ad := f.ad(t, 2, fmt.Sprintf("Велосипед %d", i), 100, domain.AdStatusActive, time.Hour)
		_, err := f.pool.ExecContext(context.Background(), `INSERT INTO review (user_id, ad_id, rating) VALUES ($1, $2, $3)`,
			f.buyer, ad, rating)
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr, rating)
		require.Equal(t, "review_rating_check", pgErr.ConstraintName, rating)
	}

	ad := f.ad(t, 2, "Велосипед 5", 100, domain.AdStatusActive, time.Hour)
	f.review(t, f.buyer, ad, 1, time.Hour)
}

func expectReviews(mock sqlmock.Sqlmock, f domain.ReviewFilter, total int, rows *sqlmock.Rows, rating float64, distribution map[int]int) {
	mock.ExpectBegin()
	mock.ExpectQuery(sellerExistsQuery).WithArgs(f.SellerID).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(countSellerReviewsQuery).WithArgs(f.SellerID, f.WithPhotos).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(total))
	mock.ExpectQuery(listSellerReviewsQuery).WithArgs(f.SellerID, f.WithPhotos, f.Sort, f.Limit, f.Offset).WillReturnRows(rows).RowsWillBeClosed()
	mock.ExpectQuery(sellerRatingQuery).WithArgs(f.SellerID).WillReturnRows(sqlmock.NewRows([]string{"rating"}).AddRow(rating))
	counts := sqlmock.NewRows([]string{"stars", "count"})
	for stars, count := range distribution {
		counts.AddRow(stars, count)
	}
	mock.ExpectQuery(sellerRatingDistributionQuery).WithArgs(f.SellerID).WillReturnRows(counts).RowsWillBeClosed()
	mock.ExpectCommit()
}
