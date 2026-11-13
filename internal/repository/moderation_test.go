package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
	"github.com/stretchr/testify/require"
)

func TestModerationRepo_ConditionalDecision(t *testing.T) {
	for _, rows := range []int64{0, 1} {
		db, mock := newSQLMock(t)
		mock.ExpectExec(decideModerationQuery).WithArgs(int32(3), int32(7), int64(2), "rejected", "reason").WillReturnResult(sqlmock.NewResult(0, rows))
		err := NewModerationRepository(db).Decide(context.Background(), 3, 7, 2, "rejected", "reason")
		if rows == 0 {
			require.ErrorIs(t, err, domain.ErrModerationConflict)
		} else {
			require.NoError(t, err)
		}
	}
}

func TestModerationRepo_QueueAndMissing(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewModerationRepository(db)
	now := time.Now()
	mock.ExpectQuery(pendingListingsQuery).WithArgs(int32(5), 20).WillReturnRows(sqlmock.NewRows([]string{"id", "title", "revision", "updated_at"}).AddRow(6, "Bike", 4, now)).RowsWillBeClosed()
	items, err := repo.ListPending(context.Background(), 5, 20)
	require.NoError(t, err)
	require.Equal(t, []domain.ModerationItem{{ID: 6, Title: "Bike", Revision: 4, UpdatedAt: now}}, items)
	mock.ExpectQuery(lockModerationRevisionQuery).WithArgs(int32(99)).WillReturnError(sql.ErrNoRows)
	_, err = repo.LockRevision(context.Background(), 99)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
}

func TestModerationFlow_RejectEditApprove(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	listings := NewListingRepository(f.pool)
	tx := NewTransactor(f.pool)
	moderation := usecase.NewModerationUsecase(NewModerationRepository(f.pool), listings, tx, []int32{f.buyer})
	id := f.ad(t, 2, "Bike", 10000, domain.AdStatusModeration, 0)
	_, err := f.pool.ExecContext(ctx, `UPDATE ad SET condition = 'used_good', address = 'Moscow' WHERE id = $1`, id)
	require.NoError(t, err)
	_, err = f.pool.ExecContext(ctx, `INSERT INTO ad_image (ad_id, user_id, url, position) VALUES ($1, $2, '/uploads/listings/test.jpg', 0)`, id, f.seller)
	require.NoError(t, err)
	snapshot, err := moderation.Get(ctx, f.buyer, id)
	require.NoError(t, err)
	require.Len(t, snapshot.Listing.Photos, 1)
	require.NoError(t, moderation.Decide(ctx, f.buyer, id, snapshot.Revision, false, "Исправьте описание"))
	sellerUC := usecase.NewListingUsecase(listings, nil, tx)
	got, err := sellerUC.Get(ctx, id, f.seller)
	require.NoError(t, err)
	require.Equal(t, "Исправьте описание", *got.RejectionReason)
	_, err = sellerUC.Get(ctx, id, 0)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
	title := "Corrected bike"
	got, err = sellerUC.Update(ctx, f.seller, id, domain.ListingFields{Title: &title}, nil)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusModeration, got.Status)
	require.Nil(t, got.RejectionReason)
	require.ErrorIs(t, moderation.Decide(ctx, f.buyer, id, snapshot.Revision, true, ""), domain.ErrModerationConflict)
	snapshot, err = moderation.Get(ctx, f.buyer, id)
	require.NoError(t, err)
	require.NoError(t, moderation.Decide(ctx, f.buyer, id, snapshot.Revision, true, ""))
	got, err = sellerUC.Get(ctx, id, 0)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusActive, got.Status)
	require.NotNil(t, got.PublishedAt)
	var decisions int
	require.NoError(t, f.pool.QueryRowContext(ctx, `SELECT count(*) FROM moderation_decision WHERE ad_id = $1 AND moderator_id = $2`, id, f.buyer).Scan(&decisions))
	require.Equal(t, 2, decisions)
	title = "Changed after approval"
	got, err = sellerUC.Update(ctx, f.seller, id, domain.ListingFields{Title: &title}, nil)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusModeration, got.Status)
}

func TestModerationFlow_ConcurrentDecisions(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	id := f.ad(t, 2, "Bike", 10000, domain.AdStatusModeration, 0)
	repo := NewModerationRepository(f.pool)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, status := range []string{domain.AdStatusActive, domain.AdStatusRejected} {
		wg.Go(func() { errs <- repo.Decide(ctx, id, f.buyer, 1, status, "reason") })
	}
	wg.Wait()
	close(errs)
	var succeeded, conflicts int
	for err := range errs {
		if err == nil {
			succeeded++
		} else if errors.Is(err, domain.ErrModerationConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, conflicts)
	var count int
	require.NoError(t, f.pool.QueryRowContext(ctx, `SELECT count(*) FROM moderation_decision WHERE ad_id = $1`, id).Scan(&count))
	require.Equal(t, 1, count)
}
