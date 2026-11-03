package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func resetRows(userID int32, expires time.Time, used any) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"user_id", "expires_at", "used_at"}).AddRow(userID, expires, used)
}

func TestPasswordReset_CreateAndGet(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewPasswordResetRepository(db)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)
	mock.ExpectExec(createPasswordResetQuery).WithArgs(hashOf("a"), int32(1), expires).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Create(ctx, hashOf("a"), 1, expires))
	for i, get := range []func(context.Context, string) (domain.PasswordReset, error){repo.Get, repo.GetForUpdate} {
		query := []string{getPasswordResetQuery, getPasswordResetForUpdateQuery}[i]
		mock.ExpectQuery(query).WithArgs(hashOf("a")).WillReturnRows(resetRows(1, expires, nil))
		reset, err := get(ctx, hashOf("a"))
		require.NoError(t, err)
		require.Equal(t, int32(1), reset.UserID)
		require.Equal(t, expires, reset.ExpiresAt)
		require.Nil(t, reset.UsedAt)
		mock.ExpectQuery(query).WithArgs(hashOf("b")).WillReturnError(sql.ErrNoRows)
		_, err = get(ctx, hashOf("b"))
		require.ErrorIs(t, err, domain.ErrInvalidResetToken)
	}
}

func TestPasswordReset_HasCreatedSince(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewPasswordResetRepository(db)
	ctx := context.Background()
	now := time.Now()
	for i, want := range []bool{false, true, false} {
		if i == 1 {
			mock.ExpectExec(createPasswordResetQuery).WithArgs(hashOf("a"), int32(1), now.Add(time.Hour)).WillReturnResult(sqlmock.NewResult(0, 1))
			require.NoError(t, repo.Create(ctx, hashOf("a"), 1, now.Add(time.Hour)))
		}
		since := now.Add(-time.Minute)
		if i == 2 {
			since = now.Add(time.Minute)
		}
		mock.ExpectQuery(hasPasswordResetSinceQuery).WithArgs(int32(1), since).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(want))
		recent, err := repo.HasCreatedSince(ctx, 1, since)
		require.NoError(t, err)
		require.Equal(t, want, recent)
	}
}

func TestPasswordReset_UseAll(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewPasswordResetRepository(db)
	ctx := context.Background()
	now := time.Now()
	mock.ExpectExec(usePasswordResetsQuery).WithArgs(int32(1)).WillReturnResult(sqlmock.NewResult(0, 2))
	require.NoError(t, repo.UseAll(ctx, 1))
	for _, token := range []string{"a", "b", "c"} {
		var used any = now
		id := int32(1)
		if token == "c" {
			used = nil
			id = 2
		}
		mock.ExpectQuery(getPasswordResetForUpdateQuery).WithArgs(hashOf(token)).WillReturnRows(resetRows(id, now.Add(time.Hour), used))
		reset, err := repo.GetForUpdate(ctx, hashOf(token))
		require.NoError(t, err)
		if token == "c" {
			require.Nil(t, reset.UsedAt)
		} else {
			require.NotNil(t, reset.UsedAt)
		}
	}
}

func TestUser_LockByID(t *testing.T) {
	db, mock := newSQLMock(t)
	users := NewUserRepository(db)
	mock.ExpectQuery(lockUserQuery).WithArgs(int32(1)).WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	require.NoError(t, users.LockByID(context.Background(), 1))
	mock.ExpectQuery(lockUserQuery).WithArgs(int32(999)).WillReturnError(sql.ErrNoRows)
	require.ErrorIs(t, users.LockByID(context.Background(), 999), domain.ErrUserNotFound)
}

func hashOf(token string) string {
	h := make([]byte, 64)
	for i := range h {
		h[i] = token[0]
	}
	return string(h)
}
