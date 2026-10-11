package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserRepo_Create_Success(t *testing.T) {
	repo := NewUserRepository()

	user, err := repo.Create(context.Background(), "a@b.ru", "hash", "ivan", "+79001234567")
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
	require.Equal(t, "a@b.ru", user.Email)
	require.Equal(t, "ivan", user.FirstName)
	require.Equal(t, "+79001234567", user.Phone)
	require.NotZero(t, user.CreatedAt)
}

func TestUserRepo_Create_EmailTaken(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.Create(context.Background(), "a@b.ru", "hash", "ivan", "+79001234567")
	require.NoError(t, err)

	_, err = repo.Create(context.Background(), "a@b.ru", "hash2", "petr", "+79007654321")
	require.ErrorIs(t, err, ErrUserExists)
}

func TestUserRepo_Create_PhoneTaken(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.Create(context.Background(), "a@b.ru", "hash", "ivan", "+79001234567")
	require.NoError(t, err)

	_, err = repo.Create(context.Background(), "c@d.ru", "hash2", "petr", "+79001234567")
	require.ErrorIs(t, err, ErrPhoneExists)
}

func TestUserRepo_Create_SameFirstNameAllowed(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.Create(context.Background(), "a@b.ru", "hash", "Иван", "+79001234567")
	require.NoError(t, err)

	_, err = repo.Create(context.Background(), "c@d.ru", "hash2", "Иван", "+79007654321")
	require.NoError(t, err)
}

func TestUserRepo_GetByEmail_Success(t *testing.T) {
	repo := NewUserRepository()

	created, err := repo.Create(context.Background(), "a@b.ru", "hash", "ivan", "+79001234567")
	require.NoError(t, err)

	user, err := repo.GetByEmail(context.Background(), "a@b.ru")
	require.NoError(t, err)
	require.Equal(t, created.ID, user.ID)
}

func TestUserRepo_GetByEmail_NotFound(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.GetByEmail(context.Background(), "nope@b.ru")
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestUserRepo_GetByPhone_Success(t *testing.T) {
	repo := NewUserRepository()

	created, err := repo.Create(context.Background(), "a@b.ru", "hash", "ivan", "+79001234567")
	require.NoError(t, err)

	user, err := repo.GetByPhone(context.Background(), "+79001234567")
	require.NoError(t, err)
	require.Equal(t, created.ID, user.ID)
}

func TestUserRepo_GetByPhone_NotFound(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.GetByPhone(context.Background(), "+70000000000")
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestUserRepo_GetByID_Success(t *testing.T) {
	repo := NewUserRepository()

	created, err := repo.Create(context.Background(), "a@b.ru", "hash", "ivan", "+79001234567")
	require.NoError(t, err)

	user, err := repo.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, user.ID)
	require.Equal(t, "a@b.ru", user.Email)
}

func TestUserRepo_GetByID_NotFound(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.GetByID(context.Background(), 999)
	require.ErrorIs(t, err, ErrUserNotFound)
}
