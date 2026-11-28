package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestOnlineConfAdminLifecycle(t *testing.T) {
	db := newTestPool(t)
	ctx := context.Background()
	repo := NewOnlineConfAdminRepository(db)

	_, err := db.ExecContext(ctx, `
		INSERT INTO user_account (first_name, email, phone_number, password_hash)
		SELECT 'Пользователь ' || n, 'user' || n || '@example.org', '+7999000000' || n, 'unused'
		FROM generate_series(1, 5) AS n
	`)
	require.NoError(t, err)

	one, four, missing := int32(1), int32(4), int32(6)
	_, err = repo.Grant(ctx, missing, nil)
	require.ErrorIs(t, err, domain.ErrUserNotFound)
	_, err = repo.Grant(ctx, 5, &missing)
	require.ErrorIs(t, err, domain.ErrUserNotFound)

	admin, err := repo.IsAdmin(ctx, 5)
	require.NoError(t, err)
	require.False(t, admin)

	created, err := repo.Grant(ctx, 5, nil)
	require.NoError(t, err)
	require.True(t, created)
	admin, err = repo.IsAdmin(ctx, 5)
	require.NoError(t, err)
	require.True(t, admin)

	created, err = repo.Grant(ctx, 5, &one)
	require.NoError(t, err)
	require.False(t, created)
	var grantedBy sql.NullInt32
	err = db.QueryRowContext(ctx, `SELECT granted_by FROM onlineconf_admin WHERE user_id = 5`).Scan(&grantedBy)
	require.NoError(t, err)
	require.False(t, grantedBy.Valid)

	_, err = db.ExecContext(ctx, `DELETE FROM onlineconf_admin WHERE user_id = 5`)
	require.NoError(t, err)
	admin, err = repo.IsAdmin(ctx, 5)
	require.NoError(t, err)
	require.False(t, admin)

	// Право удаляется вместе с аккаунтом, а выданные им права остаются без автора.
	_, err = repo.Grant(ctx, 4, nil)
	require.NoError(t, err)
	_, err = repo.Grant(ctx, 3, &four)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM user_account WHERE user_id = 4`)
	require.NoError(t, err)
	admin, err = repo.IsAdmin(ctx, 4)
	require.NoError(t, err)
	require.False(t, admin)
	err = db.QueryRowContext(ctx, `SELECT granted_by FROM onlineconf_admin WHERE user_id = 3`).Scan(&grantedBy)
	require.NoError(t, err)
	require.False(t, grantedBy.Valid)
}

func TestOnlineConfAdmin_CheckReady_MissingTable(t *testing.T) {
	db := newTestPool(t)
	ctx := context.Background()
	repo := NewOnlineConfAdminRepository(db)
	require.NoError(t, repo.CheckReady(ctx))
	_, err := db.ExecContext(ctx, `DROP TABLE onlineconf_admin`)
	require.NoError(t, err)
	require.ErrorContains(t, repo.CheckReady(ctx), "check onlineconf schema")
}
