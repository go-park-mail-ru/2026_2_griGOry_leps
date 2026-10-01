package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
)

func userCols() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "email", "password_hash", "firstname", "nickname", "phonenumber", "created_at",
	})
}

func TestUserRepo_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("a@b.ru", "hash", "Ivan", "ivan", "+79001234567").
		WillReturnRows(userCols().AddRow(int32(1), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567", time.Now()))

	user, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if user.ID != 1 || user.Email != "a@b.ru" {
		t.Errorf("user = %+v", user)
	}
}

func TestUserRepo_Create_EmailTaken(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("a@b.ru", "hash", "Ivan", "ivan", "+79001234567").
		WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"})

	_, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	if !errors.Is(err, ErrUserExists) {
		t.Errorf("err = %v, want ErrUserExists", err)
	}
}

func TestUserRepo_Create_PhoneTaken(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("a@b.ru", "hash", "Ivan", "ivan", "+79001234567").
		WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "users_phonenumber_key"})

	_, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	if !errors.Is(err, ErrPhoneExists) {
		t.Errorf("err = %v, want ErrPhoneExists", err)
	}
}

func TestUserRepo_Create_OtherError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("a@b.ru", "hash", "Ivan", "ivan", "+79001234567").
		WillReturnError(errors.New("boom"))

	_, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	if err == nil || errors.Is(err, ErrUserExists) || errors.Is(err, ErrPhoneExists) {
		t.Errorf("err = %v, want generic error", err)
	}
}

func TestUserRepo_GetByEmail_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("a@b.ru").
		WillReturnRows(userCols().AddRow(int32(1), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567", time.Now()))

	user, err := repo.GetByEmail(context.Background(), "a@b.ru")
	if err != nil || user.ID != 1 {
		t.Errorf("user = %+v, err = %v", user, err)
	}
}

func TestUserRepo_GetByEmail_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("nope@b.ru").
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.GetByEmail(context.Background(), "nope@b.ru")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}

func TestUserRepo_GetByPhone_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE phonenumber").
		WithArgs("+79001234567").
		WillReturnRows(userCols().AddRow(int32(1), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567", time.Now()))

	user, err := repo.GetByPhone(context.Background(), "+79001234567")
	if err != nil || user.ID != 1 {
		t.Errorf("user = %+v, err = %v", user, err)
	}
}

func TestUserRepo_GetByPhone_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE phonenumber").
		WithArgs("+70000000000").
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.GetByPhone(context.Background(), "+70000000000")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}

func TestUserRepo_GetByID_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(int32(1)).
		WillReturnRows(userCols().AddRow(int32(1), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567", time.Now()))

	user, err := repo.GetByID(context.Background(), 1)
	if err != nil || user.ID != 1 {
		t.Errorf("user = %+v, err = %v", user, err)
	}
}

func TestUserRepo_GetByID_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewUserRepository(mock)

	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(int32(999)).
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.GetByID(context.Background(), 999)
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}
