package repository

import (
	"context"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSessionRepo_Create_Success(t *testing.T) {
	repo := NewSessionRepository()

	err := repo.Create(context.Background(), "sess-1", 1, time.Now().Add(time.Hour))
	require.NoError(t, err)
}

func TestSessionRepo_GetByID_Success(t *testing.T) {
	repo := NewSessionRepository()

	expires := time.Now().Add(time.Hour).Truncate(time.Second)
	require.NoError(t, repo.Create(context.Background(), "sess-1", 1, expires))

	session, err := repo.GetByID(context.Background(), "sess-1")
	require.NoError(t, err)
	require.Equal(t, "sess-1", session.ID)
	require.Equal(t, int32(1), session.UserID)
}

func TestSessionRepo_GetByID_NotFound(t *testing.T) {
	repo := NewSessionRepository()

	_, err := repo.GetByID(context.Background(), "nope")
	require.ErrorIs(t, err, domain.ErrSessionNotFound)
}

func TestSessionRepo_Delete(t *testing.T) {
	repo := NewSessionRepository()

	require.NoError(t, repo.Create(context.Background(), "sess-1", 1, time.Now().Add(time.Hour)))
	require.NoError(t, repo.Delete(context.Background(), "sess-1"))

	_, err := repo.GetByID(context.Background(), "sess-1")
	require.ErrorIs(t, err, domain.ErrSessionNotFound)
}

func TestSessionRepo_Delete_Nonexistent(t *testing.T) {
	repo := NewSessionRepository()

	require.NoError(t, repo.Delete(context.Background(), "nope"))
}
