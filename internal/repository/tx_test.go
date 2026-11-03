package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestTransactor_RollbackOnError(t *testing.T) {
	db, mock := newSQLMock(t)
	ctx := context.Background()
	failure := errors.New("session failed")
	expires := time.Now().Add(time.Hour)
	mock.ExpectBegin()
	mock.ExpectQuery(createUserQuery).WithArgs("tx@b.ru", "hash", "ivan", "+79001234567").WillReturnRows(userMockRows(1, "tx@b.ru", "+79001234567"))
	mock.ExpectExec(createSessionQuery).WithArgs("sid", int32(1), expires).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()
	err := NewTransactor(db).WithinTx(ctx, func(ctx context.Context) error {
		user, err := NewUserRepository(db).Create(ctx, "tx@b.ru", "hash", "ivan", "+79001234567")
		require.NoError(t, err)
		require.NoError(t, NewSessionRepository(db).Create(ctx, "sid", user.ID, expires))
		return failure
	})
	require.ErrorIs(t, err, failure)
}

func TestTransactor_Commit(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(createUserQuery).WithArgs("tx@b.ru", "hash", "ivan", "+79001234567").WillReturnRows(userMockRows(1, "tx@b.ru", "+79001234567"))
	mock.ExpectCommit()
	err := NewTransactor(db).WithinTx(context.Background(), func(ctx context.Context) error {
		_, err := NewUserRepository(db).Create(ctx, "tx@b.ru", "hash", "ivan", "+79001234567")
		return err
	})
	require.NoError(t, err)
}
