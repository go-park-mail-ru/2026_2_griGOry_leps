package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func userMockRows(id int32, email, phone string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"user_id", "first_name", "email", "phone_number", "password_hash", "email_verified", "pending_email", "created_at", "is_moderator"}).
		AddRow(id, "ivan", email, phone, "hash", false, "", time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), false)
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

func TestUser_GetByID_IsModerator(t *testing.T) {
	db, mock := newSQLMock(t)
	rows := sqlmock.NewRows([]string{"user_id", "first_name", "email", "phone_number", "password_hash", "email_verified", "pending_email", "created_at", "is_moderator"}).
		AddRow(int32(1), "ivan", "a@b.ru", "+79001234567", "hash", true, "", time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), true)
	mock.ExpectQuery(getUserByIDQuery).WithArgs(int32(1)).WillReturnRows(rows)
	user, err := NewUserRepository(db).GetByID(context.Background(), int32(1))
	require.NoError(t, err)
	require.True(t, user.IsModerator)
}

// passArgs пропускает []int32 как есть: стандартный конвертер sqlmock slices не принимает, а pgx stdlib принимает.
type passArgs struct{}

func (passArgs) ConvertValue(v any) (driver.Value, error) { return v, nil }

func TestUser_GetCards(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.ValueConverterOption(passArgs{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() { require.NoError(t, mock.ExpectationsWereMet()) })
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{"user_id", "first_name", "avatar", "created_at", "city", "phone_verified", "phone_number"}).
		AddRow(int32(3), "Анна", "3.png", created, "Казань", true, "+79001234567").
		AddRow(int32(5), "Олег", "", created, "", false, "+79007654321")
	mock.ExpectQuery(getUserCardsQuery).WithArgs([]int32{3, 5}).WillReturnRows(rows)

	cards, err := NewUserRepository(db).GetCards(context.Background(), []int32{3, 5})

	require.NoError(t, err)
	require.Equal(t, []domain.UserCard{
		{ID: 3, FirstName: "Анна", Avatar: "3.png", CreatedAt: created, City: "Казань", PhoneVerified: true, Phone: "+79001234567"},
		{ID: 5, FirstName: "Олег", CreatedAt: created, Phone: "+79007654321"},
	}, cards)
}
