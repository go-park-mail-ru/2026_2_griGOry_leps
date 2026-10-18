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
