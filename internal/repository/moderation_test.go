package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
)

func TestModerationLifecycle(t *testing.T) {
	f := newSellerFixture(t)
	ctx := context.Background()
	users := NewUserRepository(f.pool)
	second, err := users.Create(ctx, "reviewer@goget.pw", "hash", "Мария", "+79990000003")
	require.NoError(t, err)
	_, err = f.pool.ExecContext(ctx, `UPDATE user_account SET is_moderator = true WHERE user_id IN ($1, $2)`, f.buyer, second.ID)
	require.NoError(t, err)
	profile, err := users.GetProfile(ctx, f.buyer)
	require.NoError(t, err)
	require.True(t, profile.IsModerator)

	repo := NewListingRepository(f.pool)
	tx := NewTransactor(f.pool)
	ads := usecase.NewAdUsecase(NewAdRepository(f.pool))
	storage, err := NewFileStorage(t.TempDir())
	require.NoError(t, err)
	listings := usecase.NewListingUsecase(repo, storage, tx)
	moderation := usecase.NewModerationUsecase(NewModerationRepository(f.pool), repo, tx)
	photoID, err := repo.CreatePhoto(ctx, f.seller, usecase.ListingPhotoURLPrefix+"photo.png")
	require.NoError(t, err)

	title, address, price, condition := "Стул", "Москва", "150", "used_good"
	fields := domain.ListingFields{
		CategoryID: domain.Optional[int32]{Set: true, Value: 2},
		Title:      &title,
		PhotoIDs:   &[]int32{photoID},
		Address:    domain.Optional[string]{Set: true, Value: address},
		Price:      domain.Optional[string]{Set: true, Value: price},
		Condition:  domain.Optional[string]{Set: true, Value: condition},
	}
	pending, err := listings.Create(ctx, f.seller, fields, true)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusModeration, pending.Status)
	require.NoError(t, listings.RunMaintenance(ctx, time.Now()))

	nextID := f.ad(t, 2, "Велосипед", 100, domain.AdStatusModeration, 0)
	page, err := moderation.List(ctx, f.buyer, "", 1)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, pending.ID, page.Items[0].ID)
	require.NotEmpty(t, page.NextCursor)
	next, err := moderation.List(ctx, f.buyer, page.NextCursor, 1)
	require.NoError(t, err)
	require.Len(t, next.Items, 1)
	require.Equal(t, nextID, next.Items[0].ID)
	require.Empty(t, next.NextCursor)
	_, err = moderation.List(ctx, f.seller, "", 20)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = listings.Get(ctx, pending.ID, f.buyer)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
	public, err := ads.List(ctx, domain.ListAdsParams{})
	require.NoError(t, err)
	require.Empty(t, public.Items)

	snapshot, err := moderation.Get(ctx, f.buyer, pending.ID)
	require.NoError(t, err)
	require.Equal(t, []domain.ListingPhoto{{ID: photoID, URL: usecase.ListingPhotoURLPrefix + "photo.png"}}, snapshot.Listing.Photos)
	require.ErrorIs(t, moderation.Decide(ctx, f.seller, pending.ID, snapshot.Revision, true, ""), domain.ErrForbidden)
	require.ErrorIs(t, moderation.Decide(ctx, f.buyer, pending.ID, snapshot.Revision, false, "  "), domain.ErrInvalidRejectionReason)
	changed := "Стул дубовый"
	_, err = listings.Update(ctx, f.seller, pending.ID, domain.ListingFields{Title: &changed}, nil)
	require.NoError(t, err)
	require.ErrorIs(t, moderation.Decide(ctx, f.buyer, pending.ID, snapshot.Revision, true, ""), domain.ErrModerationConflict)
	review, err := moderation.Get(ctx, f.buyer, pending.ID)
	require.NoError(t, err)
	require.Equal(t, changed, review.Listing.Title)

	reason := "Добавьте общий план"
	results := make(chan error, 2)
	for _, moderator := range []int32{f.buyer, second.ID} {
		go func() {
			results <- moderation.Decide(ctx, moderator, pending.ID, review.Revision, false, " "+reason+" ")
		}()
	}
	var saved, conflicts int
	for range 2 {
		switch err := <-results; {
		case err == nil:
			saved++
		case errors.Is(err, domain.ErrModerationConflict):
			conflicts++
		default:
			t.Fatal(err)
		}
	}
	require.Equal(t, 1, saved)
	require.Equal(t, 1, conflicts)
	turnedDown, err := listings.Get(ctx, pending.ID, f.seller)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusRejected, turnedDown.Status)
	require.NotNil(t, turnedDown.RejectionReason)
	require.Equal(t, reason, *turnedDown.RejectionReason)
	mine, err := ads.ListMine(ctx, domain.MyAdFilter{UserID: f.seller, Status: domain.AdStatusRejected})
	require.NoError(t, err)
	require.Len(t, mine.Items, 1)
	require.Equal(t, reason, mine.Items[0].RejectionReason)

	publish := true
	resubmitted, err := listings.Update(ctx, f.seller, pending.ID, domain.ListingFields{}, &publish)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusModeration, resubmitted.Status)
	require.Nil(t, resubmitted.RejectionReason)
	page, err = moderation.List(ctx, f.buyer, "", 20)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.Equal(t, pending.ID, page.Items[1].ID)
	review, err = moderation.Get(ctx, f.buyer, pending.ID)
	require.NoError(t, err)
	_, err = f.pool.ExecContext(ctx, `UPDATE user_account SET is_moderator = true WHERE user_id = $1`, f.seller)
	require.NoError(t, err)
	require.ErrorIs(t, moderation.Decide(ctx, f.seller, pending.ID, review.Revision, true, ""), domain.ErrOwnModeration)
	_, err = f.pool.ExecContext(ctx, `UPDATE user_account SET is_moderator = false WHERE user_id = $1`, f.buyer)
	require.NoError(t, err)
	require.ErrorIs(t, moderation.Decide(ctx, f.buyer, pending.ID, review.Revision, true, ""), domain.ErrForbidden)

	require.NoError(t, moderation.Decide(ctx, second.ID, pending.ID, review.Revision, true, ""))
	listed, err := listings.Get(ctx, pending.ID, f.buyer)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusActive, listed.Status)
	require.NotNil(t, listed.PublishedAt)
	public, err = ads.List(ctx, domain.ListAdsParams{})
	require.NoError(t, err)
	require.Equal(t, []int32{pending.ID}, adIDs(public.Items))
	var decisions int
	require.NoError(t, f.pool.QueryRowContext(ctx, `SELECT count(*) FROM moderation_decision WHERE ad_id = $1`, pending.ID).Scan(&decisions))
	require.Equal(t, 2, decisions)
}
