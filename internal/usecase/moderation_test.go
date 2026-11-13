package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/stretchr/testify/require"
)

type moderationRepoFake struct {
	ModerationRepository
	revision int64
	status   string
	reason   string
}

func (r *moderationRepoFake) LockRevision(context.Context, int32) (int64, error) {
	return r.revision, nil
}
func (r *moderationRepoFake) Decide(_ context.Context, _, _ int32, revision int64, status, reason string) error {
	if revision != r.revision {
		return domain.ErrModerationConflict
	}
	r.status, r.reason = status, reason
	r.revision++
	return nil
}

func TestModeration_AccessDeniedBeforeRepository(t *testing.T) {
	ctx := context.Background()
	uc := NewModerationUsecase(nil, nil, nil, []int32{7})
	for _, id := range []int32{0, 1} {
		_, err := uc.List(ctx, id, 0, 20)
		require.ErrorIs(t, err, domain.ErrForbidden)
		_, err = uc.Get(ctx, id, 1)
		require.ErrorIs(t, err, domain.ErrForbidden)
		require.ErrorIs(t, uc.Decide(ctx, id, 1, 1, true, ""), domain.ErrForbidden)
	}
	uc = NewModerationUsecase(nil, nil, nil, nil)
	require.ErrorIs(t, uc.Decide(ctx, 7, 1, 1, true, ""), domain.ErrForbidden)
}

func TestModeration_RejectValidationAndStaleDecision(t *testing.T) {
	ctx := context.Background()
	repo := &moderationRepoFake{revision: 3}
	uc := NewModerationUsecase(repo, nil, nil, []int32{7})
	for _, reason := range []string{"", "  \n", strings.Repeat("я", 501)} {
		require.ErrorIs(t, uc.Decide(ctx, 7, 1, 3, false, reason), domain.ErrInvalidRejectionReason)
	}
	require.ErrorIs(t, uc.Decide(ctx, 7, 1, 0, true, ""), domain.ErrInvalidRevision)
	require.ErrorIs(t, uc.Decide(ctx, 7, 1, 2, true, ""), domain.ErrModerationConflict)
	require.NoError(t, uc.Decide(ctx, 7, 1, 3, false, "  Исправьте описание  "))
	require.Equal(t, domain.AdStatusRejected, repo.status)
	require.Equal(t, "Исправьте описание", repo.reason)
	require.ErrorIs(t, uc.Decide(ctx, 7, 1, 3, true, ""), domain.ErrModerationConflict)
}

func TestModeration_GetIncludesPendingContent(t *testing.T) {
	e := newListingEnv()
	ctx := context.Background()
	l, err := e.uc.Create(ctx, authorID, e.fullFields(t), true)
	require.NoError(t, err)
	uc := NewModerationUsecase(&moderationRepoFake{revision: 2}, e.repo, txFake{}, []int32{otherID})
	got, err := uc.Get(ctx, otherID, l.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), got.Revision)
	require.Len(t, got.Listing.Photos, 2)
	require.False(t, got.Listing.IsOwner)
	_, err = e.uc.Get(ctx, l.ID, otherID)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
}

func TestListing_MaintenanceKeepsModeration(t *testing.T) {
	e := newListingEnv()
	ctx := context.Background()
	l, err := e.uc.Create(ctx, authorID, e.fullFields(t), true)
	require.NoError(t, err)
	require.NoError(t, e.uc.RunMaintenance(ctx, time.Now().Add(25*time.Hour)))
	got, err := e.uc.Get(ctx, l.ID, authorID)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusModeration, got.Status)
	require.Len(t, got.Photos, 2)
}

func TestListing_UpdateReservedKeepsContent(t *testing.T) {
	e := newListingEnv()
	l := e.active(t)
	e.repo.orders = map[int32]int32{l.ID: 8}
	_, err := e.uc.Update(context.Background(), authorID, l.ID, domain.ListingFields{Title: ptr("Изменено")}, nil)
	var orderErr *domain.ListingOrderError
	require.ErrorAs(t, err, &orderErr)
	require.Equal(t, int32(8), orderErr.OrderID)
	require.Equal(t, l.Title, e.repo.states[l.ID].Title)
	require.Equal(t, domain.AdStatusActive, e.repo.states[l.ID].Status)
}
