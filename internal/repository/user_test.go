package repository

import (
	"context"
	"testing"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestUser_Create_Success(t *testing.T) {
	repo := NewUserRepository()

	user, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
	require.Equal(t, "a@b.ru", user.Email)
	require.Equal(t, "Ivan", user.FirstName)
	require.Equal(t, "ivan", user.Nickname)
	require.Equal(t, "+79001234567", user.Phone)
	require.NotZero(t, user.CreatedAt)
}

func TestUser_Create_EmailTaken(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	_, err = repo.Create(context.Background(), "a@b.ru", "hash2", "Petr", "petr", "+79007654321")
	require.ErrorIs(t, err, domain.ErrUserExists)
}

func TestUser_Create_PhoneTaken(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	_, err = repo.Create(context.Background(), "c@d.ru", "hash2", "Petr", "petr", "+79001234567")
	require.ErrorIs(t, err, domain.ErrPhoneExists)
}

func TestUser_Create_NicknameTaken_CaseInsensitive(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "Ivan", "+79001234567")
	require.NoError(t, err)

	_, err = repo.Create(context.Background(), "c@d.ru", "hash2", "Petr", "IVAN", "+79007654321")
	require.ErrorIs(t, err, domain.ErrNicknameExists)

	_, err = repo.Create(context.Background(), "e@f.ru", "hash3", "Anna", "ivan", "+79009876543")
	require.ErrorIs(t, err, domain.ErrNicknameExists)
}

func TestUser_GetByEmail_Success(t *testing.T) {
	repo := NewUserRepository()

	created, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	user, err := repo.GetByEmail(context.Background(), "a@b.ru")
	require.NoError(t, err)
	require.Equal(t, created.ID, user.ID)
	require.Equal(t, "Ivan", user.FirstName)
}

func TestUser_GetByEmail_NotFound(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.GetByEmail(context.Background(), "nope@b.ru")
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestUser_GetByPhone_Success(t *testing.T) {
	repo := NewUserRepository()

	created, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	user, err := repo.GetByPhone(context.Background(), "+79001234567")
	require.NoError(t, err)
	require.Equal(t, created.ID, user.ID)
}

func TestUser_GetByPhone_NotFound(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.GetByPhone(context.Background(), "+70000000000")
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestUser_GetByID_Success(t *testing.T) {
	repo := NewUserRepository()

	created, err := repo.Create(context.Background(), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	user, err := repo.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, user.ID)
	require.Equal(t, "a@b.ru", user.Email)
}

func TestUser_GetByID_NotFound(t *testing.T) {
	repo := NewUserRepository()

	_, err := repo.GetByID(context.Background(), 999)
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}
