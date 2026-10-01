package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
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
			if got := normalizeEmail(tt.in); got != tt.want {
				t.Errorf("normalizeEmail(%q) = %q, want %q", tt.in, got, tt.want)
			}
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
			if got := normalizePhone(tt.in); got != tt.want {
				t.Errorf("normalizePhone(%q) = %q, want %q", tt.in, got, tt.want)
			}
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
		if got := isValidPhone(tt.in); got != tt.want {
			t.Errorf("isValidPhone(%q) = %v, want %v", tt.in, got, tt.want)
		}
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
			if got := isStrongPassword(tt.in); got != tt.want {
				t.Errorf("isStrongPassword(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestGenerateToken(t *testing.T) {
	tok1, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}
	if len(tok1) != 64 {
		t.Errorf("token length = %d, want 64", len(tok1))
	}
	tok2, _ := generateToken()
	if tok1 == tok2 {
		t.Error("generateToken() returned same token twice")
	}
}

func newMockUsecase(t *testing.T) (*AuthUsecase, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
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
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
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
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Register() err = %v, want %v", err, tt.wantErr)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user.ID != 1 {
		t.Errorf("user.ID = %d, want 1", user.ID)
	}
	if user.PasswordHash != "" {
		t.Error("PasswordHash должен быть очищен")
	}
	if session.ID == "" || session.UserID != 1 {
		t.Errorf("session = %+v", session)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
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
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("err = %v, want ErrEmailTaken", err)
	}
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
	if !errors.Is(err, ErrPhoneTaken) {
		t.Errorf("err = %v, want ErrPhoneTaken", err)
	}
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
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if user.ID != 1 || user.PasswordHash != "" {
		t.Errorf("user = %+v", user)
	}
	if session.ID == "" {
		t.Error("session пустая")
	}
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

	if _, _, err := uc.Login(context.Background(), "89001234567", password); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("a@b.ru").
		WillReturnError(pgx.ErrNoRows)

	_, _, err := uc.Login(context.Background(), "a@b.ru", "Secret123")
	if !errors.Is(err, ErrInvalidLogin) {
		t.Errorf("err = %v, want ErrInvalidLogin", err)
	}
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
	if !errors.Is(err, ErrInvalidLogin) {
		t.Errorf("err = %v, want ErrInvalidLogin", err)
	}
}

func TestLogout(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	mock.ExpectExec("DELETE FROM sessions").
		WithArgs("session-id").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	if err := uc.Logout(context.Background(), "session-id"); err != nil {
		t.Errorf("Logout() error = %v", err)
	}
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
	if err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	if user.ID != 1 {
		t.Errorf("user.ID = %d, want 1", user.ID)
	}
}

func TestMe_SessionNotFound(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	mock.ExpectQuery("SELECT .+ FROM sessions").
		WithArgs("nope").
		WillReturnError(pgx.ErrNoRows)

	_, err := uc.Me(context.Background(), "nope")
	if !errors.Is(err, ErrSessionExpired) {
		t.Errorf("err = %v, want ErrSessionExpired", err)
	}
}

func TestMe_SessionExpired(t *testing.T) {
	uc, mock := newMockUsecase(t)
	defer mock.Close()

	mock.ExpectQuery("SELECT .+ FROM sessions").
		WithArgs("session-id").
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at"}).
			AddRow("session-id", int32(1), time.Now().Add(-time.Hour)))

	_, err := uc.Me(context.Background(), "session-id")
	if !errors.Is(err, ErrSessionExpired) {
		t.Errorf("err = %v, want ErrSessionExpired", err)
	}
}
