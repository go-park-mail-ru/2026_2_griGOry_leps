package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
)

func newTestUsecase() *AuthUsecase {
	return NewAuthUsecase(repository.NewUserRepository(), repository.NewSessionRepository())
}

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
		{"ten digits", "9001234567", "+79001234567"},
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
		name string
		in   string
		want bool
	}{
		{"plus-7 valid", "+79001234567", true},
		{"plus-7 zeros", "+70000000000", true},
		{"8-prefix invalid", "89001234567", false},
		{"7-prefix invalid", "79001234567", false},
		{"too short", "12345", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isValidPhone(tt.in))
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"valid", "test@mail.ru", true},
		{"valid with subdomain", "user@sub.example.com", true},
		{"no at", "not-an-email", false},
		{"no domain", "a@", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isValidEmail(tt.in))
		})
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

func TestRegister_ValidationErrors(t *testing.T) {
	ctx := context.Background()

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
		{"nickname too short", "a@b.ru", "Secret123", "Ivan", "ab", "+79001234567", ErrInvalidNickname},
		{"nickname cyrillic", "a@b.ru", "Secret123", "Ivan", "иван", "+79001234567", ErrInvalidNickname},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := newTestUsecase()
			_, _, err := uc.Register(ctx, tt.email, tt.pass, tt.first, tt.nick, tt.phone)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestRegister_PasswordTooLong(t *testing.T) {
	uc := newTestUsecase()

	longPassword := "Aa1" + strings.Repeat("x", 100)
	_, _, err := uc.Register(context.Background(), "a@b.ru", longPassword, "Ivan", "ivan", "+79001234567")
	require.ErrorIs(t, err, ErrPasswordTooLong)
	require.NotErrorIs(t, err, ErrWeakPassword)
}

func TestRegister_FirstNameTooLong(t *testing.T) {
	uc := newTestUsecase()

	longName := strings.Repeat("И", 200)
	_, _, err := uc.Register(context.Background(), "a@b.ru", "Secret123", longName, "ivan", "+79001234567")
	require.ErrorIs(t, err, ErrFirstNameTooLong)
}

func TestRegister_Success(t *testing.T) {
	uc := newTestUsecase()

	user, session, err := uc.Register(context.Background(), "Test@Mail.Ru", "Secret123", "Ivan", "ivan", "89001234567")
	require.NoError(t, err)
	require.NotZero(t, user.ID)
	require.Equal(t, "test@mail.ru", user.Email)
	require.Equal(t, "Ivan", user.FirstName)
	require.Equal(t, "ivan", user.Nickname)
	require.Equal(t, "+79001234567", user.Phone)
	require.Empty(t, user.PasswordHash)
	require.NotEmpty(t, session.ID)
	require.Equal(t, user.ID, session.UserID)
}

func TestRegister_EmailTaken(t *testing.T) {
	uc := newTestUsecase()
	ctx := context.Background()

	_, _, err := uc.Register(ctx, "a@b.ru", "Secret123", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	_, _, err = uc.Register(ctx, "a@b.ru", "Secret123", "Petr", "petr", "+79007654321")
	require.ErrorIs(t, err, ErrEmailTaken)
}

func TestRegister_PhoneTaken(t *testing.T) {
	uc := newTestUsecase()
	ctx := context.Background()

	_, _, err := uc.Register(ctx, "a@b.ru", "Secret123", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	_, _, err = uc.Register(ctx, "c@d.ru", "Secret123", "Petr", "petr", "+79001234567")
	require.ErrorIs(t, err, ErrPhoneTaken)
}

func TestRegister_NicknameTaken(t *testing.T) {
	uc := newTestUsecase()
	ctx := context.Background()

	_, _, err := uc.Register(ctx, "a@b.ru", "Secret123", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	_, _, err = uc.Register(ctx, "c@d.ru", "Secret123", "Petr", "IVAN", "+79007654321")
	require.ErrorIs(t, err, ErrNicknameTaken)
}

func TestLogin_ByEmail_Success(t *testing.T) {
	uc := newTestUsecase()
	ctx := context.Background()

	_, _, err := uc.Register(ctx, "a@b.ru", "Secret123", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	user, session, err := uc.Login(ctx, "a@b.ru", "Secret123")
	require.NoError(t, err)
	require.NotZero(t, user.ID)
	require.Empty(t, user.PasswordHash)
	require.NotEmpty(t, session.ID)
}

func TestLogin_ByPhone_Success(t *testing.T) {
	uc := newTestUsecase()
	ctx := context.Background()

	_, _, err := uc.Register(ctx, "a@b.ru", "Secret123", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	_, _, err = uc.Login(ctx, "+79001234567", "Secret123")
	require.NoError(t, err)
}

func TestLogin_UserNotFound(t *testing.T) {
	uc := newTestUsecase()

	_, _, err := uc.Login(context.Background(), "a@b.ru", "Secret123")
	require.ErrorIs(t, err, ErrInvalidLogin)
}

func TestLogin_WrongPassword(t *testing.T) {
	uc := newTestUsecase()
	ctx := context.Background()

	_, _, err := uc.Register(ctx, "a@b.ru", "CorrectPass1", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	_, _, err = uc.Login(ctx, "a@b.ru", "WrongPass1")
	require.ErrorIs(t, err, ErrInvalidLogin)
}

func TestLogout(t *testing.T) {
	uc := newTestUsecase()
	ctx := context.Background()

	_, session, err := uc.Register(ctx, "a@b.ru", "Secret123", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	require.NoError(t, uc.Logout(ctx, session.ID))

	_, err = uc.Me(ctx, session.ID)
	require.ErrorIs(t, err, ErrSessionExpired)
}

func TestMe_Success(t *testing.T) {
	uc := newTestUsecase()
	ctx := context.Background()

	created, session, err := uc.Register(ctx, "a@b.ru", "Secret123", "Ivan", "ivan", "+79001234567")
	require.NoError(t, err)

	user, err := uc.Me(ctx, session.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, user.ID)
}

func TestMe_SessionNotFound(t *testing.T) {
	uc := newTestUsecase()

	_, err := uc.Me(context.Background(), "nope")
	require.ErrorIs(t, err, ErrSessionExpired)
}
