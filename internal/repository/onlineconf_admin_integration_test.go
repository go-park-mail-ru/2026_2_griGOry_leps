package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOnlineConfAdminLifecycle(t *testing.T) {
	db := newTestPool(t)
	ctx := context.Background()
	repo := NewOnlineConfAdminRepository(db)

	admin, err := repo.IsAdmin(ctx, 5)
	require.NoError(t, err)
	require.False(t, admin)

	created, err := repo.Grant(ctx, 5, nil)
	require.NoError(t, err)
	require.True(t, created)
	admin, err = repo.IsAdmin(ctx, 5)
	require.NoError(t, err)
	require.True(t, admin)

	one := int32(1)
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
