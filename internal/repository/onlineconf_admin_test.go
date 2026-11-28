package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestOnlineConfAdmin_IsAdmin(t *testing.T) {
	for _, want := range []bool{true, false} {
		db, mock := newSQLMock(t)
		mock.ExpectQuery(isOnlineConfAdminQuery).WithArgs(int32(7)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(want))
		got, err := NewOnlineConfAdminRepository(db).IsAdmin(context.Background(), 7)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}

func TestOnlineConfAdmin_IsAdmin_Error(t *testing.T) {
	db, mock := newSQLMock(t)
	want := errors.New("db down")
	mock.ExpectQuery(isOnlineConfAdminQuery).WithArgs(int32(7)).WillReturnError(want)
	_, err := NewOnlineConfAdminRepository(db).IsAdmin(context.Background(), 7)
	require.ErrorIs(t, err, want)
	require.ErrorContains(t, err, "check onlineconf admin")
}

func TestOnlineConfAdmin_Grant(t *testing.T) {
	granter := int32(1)
	for _, tc := range []struct {
		affected int64
		created  bool
		by       *int32
	}{{1, true, nil}, {0, false, &granter}} {
		db, mock := newSQLMock(t)
		mock.ExpectExec(grantOnlineConfAdminQuery).WithArgs(int32(7), tc.by).
			WillReturnResult(sqlmock.NewResult(0, tc.affected))
		created, err := NewOnlineConfAdminRepository(db).Grant(context.Background(), 7, tc.by)
		require.NoError(t, err)
		require.Equal(t, tc.created, created)
	}
}

func TestOnlineConfAdmin_Grant_Error(t *testing.T) {
	for _, rowsError := range []bool{false, true} {
		db, mock := newSQLMock(t)
		want := errors.New("db down")
		expect := mock.ExpectExec(grantOnlineConfAdminQuery).WithArgs(int32(7), nil)
		if rowsError {
			expect.WillReturnResult(sqlmock.NewErrorResult(want))
		} else {
			expect.WillReturnError(want)
		}
		created, err := NewOnlineConfAdminRepository(db).Grant(context.Background(), 7, nil)
		require.False(t, created)
		require.ErrorIs(t, err, want)
		require.ErrorContains(t, err, "grant onlineconf admin")
	}
}

func TestOnlineConfAdmin_CheckReady(t *testing.T) {
	for _, want := range []error{nil, errors.New("relation does not exist")} {
		db, mock := newSQLMock(t)
		expected := mock.ExpectExec(checkOnlineConfSchemaQuery)
		if want == nil {
			expected.WillReturnResult(sqlmock.NewResult(0, 0))
		} else {
			expected.WillReturnError(want)
		}
		err := NewOnlineConfAdminRepository(db).CheckReady(context.Background())
		require.ErrorIs(t, err, want)
		if want != nil {
			require.ErrorContains(t, err, "check onlineconf schema")
		}
	}
}
