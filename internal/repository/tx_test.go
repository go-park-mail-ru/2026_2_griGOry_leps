//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestTransactor_RollbackOnError(t *testing.T) {
	pool := newTestPool(t)
	users := NewUserRepository(pool)
	sessions := NewSessionRepository(pool)
	ctx := context.Background()
	failure := errors.New("session failed")

	err := NewTransactor(pool).WithinTx(ctx, func(ctx context.Context) error {
		user, err := users.Create(ctx, "tx@b.ru", "hash", "ivan", "+79001234567")
		require.NoError(t, err)
		require.NoError(t, sessions.Create(ctx, "sid", user.ID, time.Now().Add(time.Hour)))
		return failure
	})
	require.ErrorIs(t, err, failure)

	_, err = users.GetByEmail(ctx, "tx@b.ru")
	require.ErrorIs(t, err, domain.ErrUserNotFound)
	_, err = sessions.GetByID(ctx, "sid")
	require.ErrorIs(t, err, domain.ErrSessionNotFound)
}

func TestTransactor_Commit(t *testing.T) {
	pool := newTestPool(t)
	users := NewUserRepository(pool)
	ctx := context.Background()

	err := NewTransactor(pool).WithinTx(ctx, func(ctx context.Context) error {
		_, err := users.Create(ctx, "tx@b.ru", "hash", "ivan", "+79001234567")
		return err
	})
	require.NoError(t, err)

	_, err = users.GetByEmail(ctx, "tx@b.ru")
	require.NoError(t, err)
}
