//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/stretchr/testify/require"
)

func newTestVerificationRepository(t *testing.T) (*EmailVerificationRepository, *UserRepository, int32) {
	t.Helper()
	pool := newTestPool(t)
	users := NewUserRepository(pool)
	user, err := users.Create(context.Background(), "anna@goget.pw", "hash", "Анна", "+79990000001")
	require.NoError(t, err)
	return NewEmailVerificationRepository(pool), users, user.ID
}

func TestEmailVerification_CreateGetUse(t *testing.T) {
	repo, _, userID := newTestVerificationRepository(t)
	ctx := context.Background()
	expires := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond)

	require.NoError(t, repo.Create(ctx, hashOf("a"), userID, "anna@goget.pw", expires))

	for _, get := range []func(context.Context, string) (domain.EmailVerification, error){repo.Get, repo.GetForUpdate} {
		v, err := get(ctx, hashOf("a"))
		require.NoError(t, err)
		require.Equal(t, userID, v.UserID)
		require.Equal(t, "anna@goget.pw", v.Email)
		require.True(t, expires.Equal(v.ExpiresAt))
		require.Nil(t, v.UsedAt)

		_, err = get(ctx, hashOf("b"))
		require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	}

	require.NoError(t, repo.UseAll(ctx, userID))
	v, err := repo.Get(ctx, hashOf("a"))
	require.NoError(t, err)
	require.NotNil(t, v.UsedAt)
}

func TestEmailVerification_LastCreatedAt(t *testing.T) {
	repo, _, userID := newTestVerificationRepository(t)
	ctx := context.Background()

	last, err := repo.LastCreatedAt(ctx, userID)
	require.NoError(t, err)
	require.True(t, last.IsZero())

	require.NoError(t, repo.Create(ctx, hashOf("a"), userID, "anna@goget.pw", time.Now().Add(time.Hour)))

	last, err = repo.LastCreatedAt(ctx, userID)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now(), last, time.Minute)
}

func TestUser_EmailVerifiedResetsOnEmailChange(t *testing.T) {
	_, users, userID := newTestVerificationRepository(t)
	ctx := context.Background()

	user, err := users.GetByID(ctx, userID)
	require.NoError(t, err)
	require.False(t, user.EmailVerified)

	require.NoError(t, users.SetEmailVerified(ctx, userID))
	p, err := users.GetProfile(ctx, userID)
	require.NoError(t, err)
	require.True(t, p.EmailVerified)

	name := "Аня"
	require.NoError(t, users.UpdateProfile(ctx, userID, domain.ProfileUpdate{FirstName: &name}))
	user, err = users.GetByID(ctx, userID)
	require.NoError(t, err)
	require.True(t, user.EmailVerified)

	email := "new@goget.pw"
	require.NoError(t, users.UpdateProfile(ctx, userID, domain.ProfileUpdate{Email: &email}))
	user, err = users.GetByID(ctx, userID)
	require.NoError(t, err)
	require.False(t, user.EmailVerified)

	require.ErrorIs(t, users.SetEmailVerified(ctx, 999999), domain.ErrUserNotFound)
}

func TestUser_PendingEmail(t *testing.T) {
	_, users, userID := newTestVerificationRepository(t)
	ctx := context.Background()
	require.NoError(t, users.SetEmailVerified(ctx, userID))

	require.ErrorIs(t, users.ConfirmPendingEmail(ctx, userID), domain.ErrNoPendingEmail)

	pending := "new@goget.pw"
	require.NoError(t, users.SetPendingEmail(ctx, userID, &pending))
	p, err := users.GetProfile(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, "anna@goget.pw", p.Email)
	require.Equal(t, "new@goget.pw", p.PendingEmail)

	require.NoError(t, users.ConfirmPendingEmail(ctx, userID))
	user, err := users.GetByID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, "new@goget.pw", user.Email)
	require.Empty(t, user.PendingEmail)
	require.True(t, user.EmailVerified)

	require.NoError(t, users.SetPendingEmail(ctx, userID, &pending))
	require.NoError(t, users.SetPendingEmail(ctx, userID, nil))
	user, err = users.GetByID(ctx, userID)
	require.NoError(t, err)
	require.Empty(t, user.PendingEmail)
}

func TestUser_ConfirmPendingEmailTaken(t *testing.T) {
	_, users, userID := newTestVerificationRepository(t)
	ctx := context.Background()

	pending := "boris@goget.pw"
	require.NoError(t, users.SetPendingEmail(ctx, userID, &pending))
	_, err := users.Create(ctx, "boris@goget.pw", "hash", "Борис", "+79990000002")
	require.NoError(t, err)

	require.ErrorIs(t, users.ConfirmPendingEmail(ctx, userID), domain.ErrEmailTaken)
}
