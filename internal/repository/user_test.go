package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func userMockRows(id int32, email, phone string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"user_id", "first_name", "email", "phone_number", "password_hash", "email_verified", "pending_email", "created_at"}).
		AddRow(id, "ivan", email, phone, "hash", false, "", time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
}

func TestUser_Create_Success(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery(createUserQuery).WithArgs("a@b.ru", "hash", "ivan", "+79001234567").
		WillReturnRows(userMockRows(1, "a@b.ru", "+79001234567"))
	user, err := NewUserRepository(db).Create(context.Background(), "a@b.ru", "hash", "ivan", "+79001234567")
	require.NoError(t, err)
	require.Positive(t, user.ID)
	require.Equal(t, "a@b.ru", user.Email)
	require.Equal(t, "ivan", user.FirstName)
	require.Equal(t, "+79001234567", user.Phone)
	require.NotZero(t, user.CreatedAt)
}

func TestUser_Create_SameFirstNameAllowed(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUserRepository(db)
	for i, email := range []string{"a@b.ru", "c@d.ru"} {
		phone := []string{"+79001234567", "+79007654321"}[i]
		mock.ExpectQuery(createUserQuery).WithArgs(email, "hash", "ivan", phone).
			WillReturnRows(userMockRows(int32(i+1), email, phone))
		_, err := repo.Create(context.Background(), email, "hash", "ivan", phone)
		require.NoError(t, err)
	}
}

func TestUser_Create_EmailTaken(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery(createUserQuery).WithArgs("a@b.ru", "hash", "ivan", "+79001234567").
		WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "user_account_email_key"})
	_, err := NewUserRepository(db).Create(context.Background(), "a@b.ru", "hash", "ivan", "+79001234567")
	require.ErrorIs(t, err, domain.ErrEmailTaken)
}

func TestUser_Create_PhoneTaken(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery(createUserQuery).WithArgs("a@b.ru", "hash", "ivan", "+79001234567").
		WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "user_account_phone_number_key"})
	_, err := NewUserRepository(db).Create(context.Background(), "a@b.ru", "hash", "ivan", "+79001234567")
	require.ErrorIs(t, err, domain.ErrPhoneTaken)
}

func TestUser_GetByEmail_Success(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery(getUserByEmailQuery).WithArgs("a@b.ru").WillReturnRows(userMockRows(1, "a@b.ru", "+79001234567"))
	user, err := NewUserRepository(db).GetByEmail(context.Background(), "a@b.ru")
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
}

func TestUser_GetByEmail_NotFound(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery(getUserByEmailQuery).WithArgs("a@b.ru").WillReturnError(sql.ErrNoRows)
	_, err := NewUserRepository(db).GetByEmail(context.Background(), "a@b.ru")
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestUser_GetByPhone_Success(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery(getUserByPhoneQuery).WithArgs("+79001234567").WillReturnRows(userMockRows(1, "a@b.ru", "+79001234567"))
	user, err := NewUserRepository(db).GetByPhone(context.Background(), "+79001234567")
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
}

func TestUser_GetByPhone_NotFound(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery(getUserByPhoneQuery).WithArgs("+79001234567").WillReturnError(sql.ErrNoRows)
	_, err := NewUserRepository(db).GetByPhone(context.Background(), "+79001234567")
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestUser_GetByID_Success(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery(getUserByIDQuery).WithArgs(int32(1)).WillReturnRows(userMockRows(1, "a@b.ru", "+79001234567"))
	user, err := NewUserRepository(db).GetByID(context.Background(), int32(1))
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
	require.Equal(t, "a@b.ru", user.Email)
}

func TestUser_GetByID_NotFound(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery(getUserByIDQuery).WithArgs(int32(1)).WillReturnError(sql.ErrNoRows)
	_, err := NewUserRepository(db).GetByID(context.Background(), int32(1))
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}
