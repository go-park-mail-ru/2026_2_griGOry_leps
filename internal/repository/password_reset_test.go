//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/stretchr/testify/require"
)

func newTestResetRepository(t *testing.T) (*PasswordResetRepository, *UserRepository, int32) {
	t.Helper()
	pool := newTestPool(t)
	users := NewUserRepository(pool)
	user, err := users.Create(context.Background(), "anna@goget.pw", "hash", "Анна", "+79990000001")
	require.NoError(t, err)
	return NewPasswordResetRepository(pool), users, user.ID
}

func TestPasswordReset_CreateAndGet(t *testing.T) {
	repo, _, userID := newTestResetRepository(t)
	ctx := context.Background()
	expires := time.Now().Add(30 * time.Minute).Truncate(time.Microsecond)

	require.NoError(t, repo.Create(ctx, hashOf("a"), userID, expires))

	for _, get := range []func(context.Context, string) (domain.PasswordReset, error){repo.Get, repo.GetForUpdate} {
		reset, err := get(ctx, hashOf("a"))
		require.NoError(t, err)
		require.Equal(t, userID, reset.UserID)
		require.True(t, expires.Equal(reset.ExpiresAt))
		require.Nil(t, reset.UsedAt)

		_, err = get(ctx, hashOf("b"))
		require.ErrorIs(t, err, domain.ErrInvalidResetToken)
	}
}

func TestPasswordReset_HasCreatedSince(t *testing.T) {
	repo, _, userID := newTestResetRepository(t)
	ctx := context.Background()

	recent, err := repo.HasCreatedSince(ctx, userID, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	require.False(t, recent)

	require.NoError(t, repo.Create(ctx, hashOf("a"), userID, time.Now().Add(time.Hour)))

	recent, err = repo.HasCreatedSince(ctx, userID, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	require.True(t, recent)

	recent, err = repo.HasCreatedSince(ctx, userID, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.False(t, recent)
}

func TestPasswordReset_UseAll(t *testing.T) {
	repo, users, userID := newTestResetRepository(t)
	ctx := context.Background()
	other, err := users.Create(ctx, "oleg@goget.pw", "hash", "Олег", "+79990000002")
	require.NoError(t, err)
	for token, id := range map[string]int32{"a": userID, "b": userID, "c": other.ID} {
		require.NoError(t, repo.Create(ctx, hashOf(token), id, time.Now().Add(time.Hour)))
	}

	require.NoError(t, repo.UseAll(ctx, userID))

	for _, token := range []string{"a", "b"} {
		reset, err := repo.GetForUpdate(ctx, hashOf(token))
		require.NoError(t, err)
		require.NotNil(t, reset.UsedAt, token)
	}
	reset, err := repo.GetForUpdate(ctx, hashOf("c"))
	require.NoError(t, err)
	require.Nil(t, reset.UsedAt)
}

func TestUser_LockByID(t *testing.T) {
	_, users, userID := newTestResetRepository(t)

	require.NoError(t, users.LockByID(context.Background(), userID))
	require.ErrorIs(t, users.LockByID(context.Background(), 999), domain.ErrUserNotFound)
}

func hashOf(token string) string {
	h := make([]byte, 64)
	for i := range h {
		h[i] = token[0]
	}
	return string(h)
}
