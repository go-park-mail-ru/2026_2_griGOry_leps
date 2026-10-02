package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

func TestSessionRepo_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewSessionRepository(mock)

	mock.ExpectExec("INSERT INTO sessions").
		WithArgs("sess-1", int32(1), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Create(context.Background(), "sess-1", 1, time.Now().Add(time.Hour))
	require.NoError(t, err)
}

func TestSessionRepo_GetByID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewSessionRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM sessions").
		WithArgs("sess-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at"}).
			AddRow("sess-1", int32(1), time.Now().Add(time.Hour)))

	session, err := repo.GetByID(context.Background(), "sess-1")
	require.NoError(t, err)
	require.Equal(t, "sess-1", session.ID)
	require.Equal(t, int32(1), session.UserID)
}

func TestSessionRepo_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewSessionRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM sessions").
		WithArgs("nope").
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByID(context.Background(), "nope")
	require.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionRepo_Delete(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewSessionRepository(mock)

	mock.ExpectExec("DELETE FROM sessions").
		WithArgs("sess-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.Delete(context.Background(), "sess-1")
	require.NoError(t, err)
}
