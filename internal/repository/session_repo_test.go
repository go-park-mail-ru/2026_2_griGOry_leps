package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
)

func TestSessionRepo_Create_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSessionRepository(mock)

	mock.ExpectExec("INSERT INTO sessions").
		WithArgs("sess-1", int32(1), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := repo.Create(context.Background(), "sess-1", 1, time.Now().Add(time.Hour)); err != nil {
		t.Errorf("Create() error = %v", err)
	}
}

func TestSessionRepo_GetByID_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSessionRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM sessions").
		WithArgs("sess-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at"}).
			AddRow("sess-1", int32(1), time.Now().Add(time.Hour)))

	session, err := repo.GetByID(context.Background(), "sess-1")
	if err != nil || session.ID != "sess-1" || session.UserID != 1 {
		t.Errorf("session = %+v, err = %v", session, err)
	}
}

func TestSessionRepo_GetByID_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSessionRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM sessions").
		WithArgs("nope").
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.GetByID(context.Background(), "nope")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("err = %v, want ErrSessionNotFound", err)
	}
}

func TestSessionRepo_Delete(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewSessionRepository(mock)

	mock.ExpectExec("DELETE FROM sessions").
		WithArgs("sess-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	if err := repo.Delete(context.Background(), "sess-1"); err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}
