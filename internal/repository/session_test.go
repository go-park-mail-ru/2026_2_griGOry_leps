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

func TestSession_Create_Success(t *testing.T) {
	db, mock := newSQLMock(t)
	expires := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec(createSessionQuery).WithArgs("sess-1", int32(1), expires).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, NewSessionRepository(db).Create(context.Background(), "sess-1", 1, expires))
}

func TestSession_GetByID_Success(t *testing.T) {
	db, mock := newSQLMock(t)
	expires := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(getSessionQuery).WithArgs("sess-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "expires_at"}).AddRow("sess-1", 1, expires))
	session, err := NewSessionRepository(db).GetByID(context.Background(), "sess-1")
	require.NoError(t, err)
	require.Equal(t, "sess-1", session.ID)
	require.Equal(t, int32(1), session.UserID)
}

func TestSession_GetByID_NotFound(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery(getSessionQuery).WithArgs("nope").WillReturnError(sql.ErrNoRows)
	_, err := NewSessionRepository(db).GetByID(context.Background(), "nope")
	require.ErrorIs(t, err, domain.ErrSessionNotFound)
}

func TestSession_Delete(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectExec(deleteSessionQuery).WithArgs("sess-1").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, NewSessionRepository(db).Delete(context.Background(), "sess-1"))
}

func TestSession_Delete_Nonexistent(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectExec(deleteSessionQuery).WithArgs("nope").WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, NewSessionRepository(db).Delete(context.Background(), "nope"))
}

func TestSession_DeleteOthers(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectExec(deleteOtherSessionsQuery).WithArgs(int32(1), "current").WillReturnResult(sqlmock.NewResult(0, 2))
	require.NoError(t, NewSessionRepository(db).DeleteOthers(context.Background(), int32(1), "current"))
}

func TestSession_DeleteAll(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectExec(deleteUserSessionsQuery).WithArgs(int32(1)).WillReturnResult(sqlmock.NewResult(0, 2))
	require.NoError(t, NewSessionRepository(db).DeleteAll(context.Background(), int32(1)))
}
