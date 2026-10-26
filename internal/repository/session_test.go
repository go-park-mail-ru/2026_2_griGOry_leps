//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSession_Create_Success(t *testing.T) {
	repo, userID := newTestSessionRepository(t)

	err := repo.Create(context.Background(), "sess-1", userID, time.Now().Add(time.Hour))
	require.NoError(t, err)
}

func TestSession_GetByID_Success(t *testing.T) {
	repo, userID := newTestSessionRepository(t)

	expires := time.Now().Add(time.Hour).Truncate(time.Second)
	require.NoError(t, repo.Create(context.Background(), "sess-1", userID, expires))

	session, err := repo.GetByID(context.Background(), "sess-1")
	require.NoError(t, err)
	require.Equal(t, "sess-1", session.ID)
	require.Equal(t, userID, session.UserID)
}

func TestSession_GetByID_NotFound(t *testing.T) {
	repo, _ := newTestSessionRepository(t)

	_, err := repo.GetByID(context.Background(), "nope")
	require.ErrorIs(t, err, domain.ErrSessionNotFound)
}

func TestSession_Delete(t *testing.T) {
	repo, userID := newTestSessionRepository(t)

	require.NoError(t, repo.Create(context.Background(), "sess-1", userID, time.Now().Add(time.Hour)))
	require.NoError(t, repo.Delete(context.Background(), "sess-1"))

	_, err := repo.GetByID(context.Background(), "sess-1")
	require.ErrorIs(t, err, domain.ErrSessionNotFound)
}

func TestSession_Delete_Nonexistent(t *testing.T) {
	repo, _ := newTestSessionRepository(t)

	require.NoError(t, repo.Delete(context.Background(), "nope"))
}

func TestSession_DeleteOthers(t *testing.T) {
	repo, userID := newTestSessionRepository(t)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)
	require.NoError(t, repo.Create(ctx, "current", userID, expires))
	require.NoError(t, repo.Create(ctx, "laptop", userID, expires))
	require.NoError(t, repo.Create(ctx, "phone", userID, expires))

	require.NoError(t, repo.DeleteOthers(ctx, userID, "current"))

	_, err := repo.GetByID(ctx, "current")
	require.NoError(t, err)
	for _, id := range []string{"laptop", "phone"} {
		_, err := repo.GetByID(ctx, id)
		require.ErrorIs(t, err, domain.ErrSessionNotFound, id)
	}
}

func TestSession_DeleteAll(t *testing.T) {
	repo, userID := newTestSessionRepository(t)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)
	require.NoError(t, repo.Create(ctx, "laptop", userID, expires))
	require.NoError(t, repo.Create(ctx, "phone", userID, expires))

	require.NoError(t, repo.DeleteAll(ctx, userID))

	for _, id := range []string{"laptop", "phone"} {
		_, err := repo.GetByID(ctx, id)
		require.ErrorIs(t, err, domain.ErrSessionNotFound, id)
	}
}
