package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
)

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"already lower", "test@mail.ru", "test@mail.ru"},
		{"upper", "TEST@MAIL.RU", "test@mail.ru"},
		{"with spaces", "  Test@Mail.Ru  ", "test@mail.ru"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, normalizeEmail(tt.in))
		})
	}
}

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"8-prefix", "89001234567", "+79001234567"},
		{"7-prefix", "79001234567", "+79001234567"},
		{"plus-7", "+79001234567", "+79001234567"},
		{"formatted", "+7 900 123-45-67", "+79001234567"},
		{"other country", "+3801234567890", "+3801234567890"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, normalizePhone(tt.in))
		})
	}
}

func TestIsValidPhone(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"+79001234567", true},
		{"89001234567", true},
		{"+7 900 123-45-67", true},
		{"12345", false},
		{"", false},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, isValidPhone(tt.in))
	}
}

func TestIsStrongPassword(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"valid", "Secret123", true},
		{"valid cyrillic", "Пароль123", true},
		{"too short", "Sec1", false},
		{"no upper", "secret123", false},
		{"no lower", "SECRET123", false},
		{"no digit", "SecretAbc", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isStrongPassword(tt.in))
		})
	}
}

func TestGenerateToken(t *testing.T) {
	tok1, err := generateToken()
	require.NoError(t, err)
	require.Len(t, tok1, 64)

	tok2, err := generateToken()
	require.NoError(t, err)
	require.NotEqual(t, tok1, tok2)
}

func newMockUsecase(t *testing.T) (*AuthUsecase, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	uc := NewAuthUsecase(
		mock,
		repository.NewUserRepository(mock),
		repository.NewSessionRepository(mock),
	)
	return uc, mock
}

func userRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "email", "password_hash", "firstname", "nickname", "phonenumber", "created_at",
	})
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	return string(h)
}

func TestRegister_ValidationErrors(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	tests := []struct {
		name    string
		email   string
		pass    string
		first   string
		nick    string
		phone   string
		wantErr error
	}{
		{"bad email", "not-an-email", "Secret123", "Ivan", "ivan", "+79001234567", ErrInvalidEmail},
		{"empty first name", "a@b.ru", "Secret123", "  ", "ivan", "+79001234567", ErrMissingFirstName},
		{"empty nickname", "a@b.ru", "Secret123", "Ivan", "  ", "+79001234567", ErrMissingNickname},
		{"bad phone", "a@b.ru", "Secret123", "Ivan", "ivan", "123", ErrInvalidPhone},
		{"weak password", "a@b.ru", "short", "Ivan", "ivan", "+79001234567", ErrWeakPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := uc.Register(context.Background(), tt.email, tt.pass, tt.first, tt.nick, tt.phone)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegister_Success(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	password := "Secret123"
	hash := mustHash(t, password)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO users").
		WithArgs("test@mail.ru", pgxmock.AnyArg(), "Ivan", "ivan", "+79001234567").
		WillReturnRows(userRows().
			AddRow(int32(1), "test@mail.ru", hash, "Ivan", "ivan", "+79001234567", time.Now()))
	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(pgxmock.AnyArg(), int32(1), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	user, session, err := uc.Register(context.Background(), "Test@Mail.Ru", password, "Ivan", "ivan", "89001234567")
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
	require.Empty(t, user.PasswordHash)
	require.NotEmpty(t, session.ID)
	require.Equal(t, int32(1), session.UserID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegister_EmailTaken(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO users").
		WithArgs("a@b.ru", pgxmock.AnyArg(), "Ivan", "ivan", "+79001234567").
		WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"})
	mock.ExpectRollback()

	_, _, err := uc.Register(context.Background(), "a@b.ru", "Secret123", "Ivan", "ivan", "+79001234567")
	require.ErrorIs(t, err, ErrEmailTaken)
}

func TestRegister_PhoneTaken(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO users").
		WithArgs("a@b.ru", pgxmock.AnyArg(), "Ivan", "ivan", "+79001234567").
		WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "users_phonenumber_key"})
	mock.ExpectRollback()

	_, _, err := uc.Register(context.Background(), "a@b.ru", "Secret123", "Ivan", "ivan", "+79001234567")
	require.ErrorIs(t, err, ErrPhoneTaken)
}

func TestLogin_ByEmail_Success(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	password := "Secret123"
	hash := mustHash(t, password)

	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("test@mail.ru").
		WillReturnRows(userRows().
			AddRow(int32(1), "test@mail.ru", hash, "Ivan", "ivan", "+79001234567", time.Now()))
	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(pgxmock.AnyArg(), int32(1), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	user, session, err := uc.Login(context.Background(), "Test@Mail.Ru", password)
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
	require.Empty(t, user.PasswordHash)
	require.NotEmpty(t, session.ID)
}

func TestLogin_ByPhone_Success(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	password := "Secret123"
	hash := mustHash(t, password)

	mock.ExpectQuery("SELECT .+ FROM users WHERE phonenumber").
		WithArgs("+79001234567").
		WillReturnRows(userRows().
			AddRow(int32(1), "a@b.ru", hash, "Ivan", "ivan", "+79001234567", time.Now()))
	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(pgxmock.AnyArg(), int32(1), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	_, _, err := uc.Login(context.Background(), "89001234567", password)
	require.NoError(t, err)
}

func TestLogin_UserNotFound(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("a@b.ru").
		WillReturnError(pgx.ErrNoRows)

	_, _, err := uc.Login(context.Background(), "a@b.ru", "Secret123")
	require.ErrorIs(t, err, ErrInvalidLogin)
}

func TestLogin_WrongPassword(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	hash := mustHash(t, "CorrectPass1")

	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("a@b.ru").
		WillReturnRows(userRows().
			AddRow(int32(1), "a@b.ru", hash, "Ivan", "ivan", "+79001234567", time.Now()))

	_, _, err := uc.Login(context.Background(), "a@b.ru", "WrongPass1")
	require.ErrorIs(t, err, ErrInvalidLogin)
}

func TestLogout(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	mock.ExpectExec("DELETE FROM sessions").
		WithArgs("session-id").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	require.NoError(t, uc.Logout(context.Background(), "session-id"))
}

func TestMe_Success(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	mock.ExpectQuery("SELECT .+ FROM sessions").
		WithArgs("session-id").
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at"}).
			AddRow("session-id", int32(1), time.Now().Add(time.Hour)))
	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(int32(1)).
		WillReturnRows(userRows().
			AddRow(int32(1), "a@b.ru", "hash", "Ivan", "ivan", "+79001234567", time.Now()))

	user, err := uc.Me(context.Background(), "session-id")
	require.NoError(t, err)
	require.Equal(t, int32(1), user.ID)
}

func TestMe_SessionNotFound(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	mock.ExpectQuery("SELECT .+ FROM sessions").
		WithArgs("nope").
		WillReturnError(pgx.ErrNoRows)

	_, err := uc.Me(context.Background(), "nope")
	require.ErrorIs(t, err, ErrSessionExpired)
}

func TestMe_SessionExpired(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	mock.ExpectQuery("SELECT .+ FROM sessions").
		WithArgs("session-id").
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at"}).
			AddRow("session-id", int32(1), time.Now().Add(-time.Hour)))

	_, err := uc.Me(context.Background(), "session-id")
	require.ErrorIs(t, err, ErrSessionExpired)
}
