package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

func userCols() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "email", "password_hash", "firstname", "nickname", "phonenumber", "created_at",
	})
}

func TestUserRepo_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("a@b.ru", "hash", "Ivan", "ivan", "+79001234567").
		WillReturnRows(userCols().AddRow(int32(1), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567", time.Now()))

	user, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
	require.Equal(t, "a@b.ru", user.Email)
}

func TestUserRepo_Create_EmailTaken(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("a@b.ru", "hash", "Ivan", "ivan", "+79001234567").
		WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"})

	_, err = repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	require.ErrorIs(t, err, ErrUserExists)
}

func TestUserRepo_Create_PhoneTaken(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("a@b.ru", "hash", "Ivan", "ivan", "+79001234567").
		WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "users_phonenumber_key"})

	_, err = repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	require.ErrorIs(t, err, ErrPhoneExists)
}

func TestUserRepo_Create_OtherError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("a@b.ru", "hash", "Ivan", "ivan", "+79001234567").
		WillReturnError(errors.New("boom"))

	_, err = repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrUserExists)
	require.NotErrorIs(t, err, ErrPhoneExists)
}

func TestUserRepo_GetByEmail_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("a@b.ru").
		WillReturnRows(userCols().AddRow(int32(1), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567", time.Now()))

	user, err := repo.GetByEmail(context.Background(), "a@b.ru")
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
}

func TestUserRepo_GetByEmail_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("nope@b.ru").
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByEmail(context.Background(), "nope@b.ru")
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestUserRepo_GetByPhone_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE phonenumber").
		WithArgs("+79001234567").
		WillReturnRows(userCols().AddRow(int32(1), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567", time.Now()))

	user, err := repo.GetByPhone(context.Background(), "+79001234567")
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
}

func TestUserRepo_GetByPhone_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE phonenumber").
		WithArgs("+70000000000").
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByPhone(context.Background(), "+70000000000")
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestUserRepo_GetByID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(int32(1)).
		WillReturnRows(userCols().AddRow(int32(1), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567", time.Now()))

	user, err := repo.GetByID(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
}

func TestUserRepo_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(int32(999)).
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByID(context.Background(), 999)
	require.ErrorIs(t, err, ErrUserNotFound)
}
