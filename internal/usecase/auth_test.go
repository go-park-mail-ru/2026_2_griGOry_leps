package usecase

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=repository.go -destination=repository_mock_test.go -package=usecase

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type authTestEnv struct {
	*AuthUsecase
	users    *MockUserRepository
	sessions *MockSessionRepository
	verifier *MockEmailVerifier
	tx       *MockTransactor
}

func newTestUsecase(t *testing.T) *authTestEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	e := &authTestEnv{users: NewMockUserRepository(ctrl), sessions: NewMockSessionRepository(ctrl), verifier: NewMockEmailVerifier(ctrl), tx: NewMockTransactor(ctrl)}
	e.AuthUsecase = NewAuthUsecase(e.users, e.sessions, e.verifier, e.tx)
	return e
}

func expectTransaction(tx *MockTransactor) *gomock.Call {
	return tx.EXPECT().WithinTx(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
}

func passwordHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	return string(hash)
}

func expectRegistration(t *testing.T, e *authTestEnv, user domain.User, password string) {
	t.Helper()
	expectTransaction(e.tx)
	e.users.EXPECT().Create(gomock.Any(), user.Email, gomock.Cond(func(hash string) bool { return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil }), user.FirstName, user.Phone).Return(user, nil)
	e.sessions.EXPECT().Create(gomock.Any(), gomock.Cond(func(id string) bool { return len(id) == 64 }), user.ID, gomock.Cond(func(exp time.Time) bool { return exp.After(time.Now()) })).Return(nil)
	e.verifier.EXPECT().IssueLink(gomock.Any(), user.ID, user.Email).Return(func() {}, nil)
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
		phone   string
		wantErr error
	}{
		{"bad email", "not-an-email", "Secret123", "ivan", "+79001234567", domain.ErrInvalidEmail},
		{"empty first name", "a@b.ru", "Secret123", "  ", "+79001234567", domain.ErrMissingFirstName},
		{"bad phone", "a@b.ru", "Secret123", "ivan", "123", domain.ErrInvalidPhone},
		{"weak password", "a@b.ru", "short", "ivan", "+79001234567", domain.ErrWeakPassword},
		{"first name too short", "a@b.ru", "Secret123", "И", "+79001234567", domain.ErrInvalidFirstName},
		{"first name too long", "a@b.ru", "Secret123", strings.Repeat("и", 51), "+79001234567", domain.ErrInvalidFirstName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := newTestUsecase(t)
			_, _, err := uc.Register(ctx, tt.email, tt.pass, tt.first, tt.phone)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestRegister_PasswordTooLong(t *testing.T) {
	uc := newTestUsecase(t)

	longPassword := "Aa1" + strings.Repeat("x", 100)
	_, _, err := uc.Register(context.Background(), "a@b.ru", longPassword, "ivan", "+79001234567")
	require.ErrorIs(t, err, domain.ErrPasswordTooLong)
	require.NotErrorIs(t, err, domain.ErrWeakPassword)
}

func TestRegister_Success(t *testing.T) {
	uc := newTestUsecase(t)
	expectRegistration(t, uc, domain.User{ID: 1, Email: "test@mail.ru", FirstName: "ivan", Phone: "+79001234567"}, "Secret123")

	user, session, err := uc.Register(context.Background(), "Test@Mail.Ru", "Secret123", "ivan", "89001234567")
	require.NoError(t, err)
	require.NotZero(t, user.ID)
	require.Equal(t, "test@mail.ru", user.Email)
	require.Equal(t, "ivan", user.FirstName)
	require.Equal(t, "+79001234567", user.Phone)
	require.Empty(t, user.PasswordHash)
	require.NotEmpty(t, session.ID)
	require.Equal(t, user.ID, session.UserID)
}

func TestRegister_EmailTaken(t *testing.T) {
	uc := newTestUsecase(t)
	expectTransaction(uc.tx)
	uc.users.EXPECT().Create(gomock.Any(), "a@b.ru", gomock.Any(), "petr", "+79007654321").Return(domain.User{}, domain.ErrEmailTaken)
	_, _, err := uc.Register(context.Background(), "a@b.ru", "Secret123", "petr", "+79007654321")
	require.ErrorIs(t, err, domain.ErrEmailTaken)
}

func TestRegister_PhoneTaken(t *testing.T) {
	uc := newTestUsecase(t)
	expectTransaction(uc.tx)
	uc.users.EXPECT().Create(gomock.Any(), "c@d.ru", gomock.Any(), "petr", "+79001234567").Return(domain.User{}, domain.ErrPhoneTaken)
	_, _, err := uc.Register(context.Background(), "c@d.ru", "Secret123", "petr", "+79001234567")
	require.ErrorIs(t, err, domain.ErrPhoneTaken)
}

func TestRegister_FirstNameAllowedChars(t *testing.T) {
	for _, name := range []string{"Иван", "ivan", "Ёжик_2026", "ivan.bike", "Ян", "Иван Петров", "Анна-Мария"} {
		t.Run(name, func(t *testing.T) {
			uc := newTestUsecase(t)
			expectRegistration(t, uc, domain.User{ID: 1, Email: "a@b.ru", FirstName: name, Phone: "+79001234567"}, "Secret123")
			_, _, err := uc.Register(context.Background(), "a@b.ru", "Secret123", name, "+79001234567")
			require.NoError(t, err)
		})
	}
}

func TestRegister_FirstNameSpacesNormalized(t *testing.T) {
	uc := newTestUsecase(t)
	expectRegistration(t, uc, domain.User{ID: 1, Email: "a@b.ru", FirstName: "Иван Петров", Phone: "+79001234567"}, "Secret123")

	user, _, err := uc.Register(context.Background(), "a@b.ru", "Secret123", "  Иван   Петров  ", "+79001234567")
	require.NoError(t, err)
	require.Equal(t, "Иван Петров", user.FirstName)
}

func TestRegister_FirstNameNotUnique(t *testing.T) {
	uc := newTestUsecase(t)
	expectRegistration(t, uc, domain.User{ID: 1, Email: "a@b.ru", FirstName: "Иван", Phone: "+79001234567"}, "Secret123")
	expectRegistration(t, uc, domain.User{ID: 2, Email: "c@d.ru", FirstName: "Иван", Phone: "+79007654321"}, "Secret123")
	ctx := context.Background()

	_, _, err := uc.Register(ctx, "a@b.ru", "Secret123", "Иван", "+79001234567")
	require.NoError(t, err)

	_, _, err = uc.Register(ctx, "c@d.ru", "Secret123", "Иван", "+79007654321")
	require.NoError(t, err)
}

func TestLogin_ByEmail_Success(t *testing.T) {
	uc := newTestUsecase(t)
	hash := passwordHash(t, "Secret123")
	uc.users.EXPECT().GetByEmail(gomock.Any(), "a@b.ru").Return(domain.User{ID: 7, PasswordHash: hash}, nil)
	expectTransaction(uc.tx)
	uc.users.EXPECT().GetPasswordHashForUpdate(gomock.Any(), int32(7)).Return(hash, nil)
	uc.sessions.EXPECT().Create(gomock.Any(), gomock.Any(), int32(7), gomock.Any()).Return(nil)
	user, session, err := uc.Login(context.Background(), "a@b.ru", "Secret123")
	require.NoError(t, err)
	require.Equal(t, int32(7), user.ID)
	require.Empty(t, user.PasswordHash)
	require.Len(t, session.ID, 64)
	require.Equal(t, user.ID, session.UserID)
}

func TestLogin_ByPhone_Success(t *testing.T) {
	uc := newTestUsecase(t)
	hash := passwordHash(t, "Secret123")
	uc.users.EXPECT().GetByPhone(gomock.Any(), "+79001234567").Return(domain.User{ID: 7, PasswordHash: hash}, nil)
	expectTransaction(uc.tx)
	uc.users.EXPECT().GetPasswordHashForUpdate(gomock.Any(), int32(7)).Return(hash, nil)
	uc.sessions.EXPECT().Create(gomock.Any(), gomock.Any(), int32(7), gomock.Any()).Return(nil)
	user, session, err := uc.Login(context.Background(), "+79001234567", "Secret123")
	require.NoError(t, err)
	require.Equal(t, int32(7), user.ID)
	require.Empty(t, user.PasswordHash)
	require.Len(t, session.ID, 64)
	require.Equal(t, user.ID, session.UserID)
}

func TestLogin_UserNotFound(t *testing.T) {
	uc := newTestUsecase(t)
	uc.users.EXPECT().GetByEmail(gomock.Any(), "a@b.ru").Return(domain.User{}, domain.ErrUserNotFound)

	_, _, err := uc.Login(context.Background(), "a@b.ru", "Secret123")
	require.ErrorIs(t, err, domain.ErrInvalidLogin)
}

func TestLogin_UnknownUserTakesAsLongAsWrongPassword(t *testing.T) {
	uc := newTestUsecase(t)
	ctx := context.Background()

	uc.users.EXPECT().GetByEmail(gomock.Any(), "a@b.ru").Return(domain.User{PasswordHash: passwordHash(t, "CorrectPass1")}, nil).Times(3)
	uc.users.EXPECT().GetByEmail(gomock.Any(), "nobody@b.ru").Return(domain.User{}, domain.ErrUserNotFound).Times(3)

	fastest := func(login string) time.Duration {
		best := time.Duration(math.MaxInt64)
		for range 3 {
			start := time.Now()
			_, _, err := uc.Login(ctx, login, "WrongPass1")
			require.ErrorIs(t, err, domain.ErrInvalidLogin)
			best = min(best, time.Since(start))
		}
		return best
	}

	wrongPassword := fastest("a@b.ru")
	unknownUser := fastest("nobody@b.ru")
	require.Greater(t, unknownUser, wrongPassword/2)
}

func TestLogin_WrongPassword(t *testing.T) {
	uc := newTestUsecase(t)
	uc.users.EXPECT().GetByEmail(gomock.Any(), "a@b.ru").Return(domain.User{PasswordHash: passwordHash(t, "CorrectPass1")}, nil)
	_, _, err := uc.Login(context.Background(), "a@b.ru", "WrongPass1")
	require.ErrorIs(t, err, domain.ErrInvalidLogin)
}

func TestLogout(t *testing.T) {
	uc := newTestUsecase(t)
	uc.sessions.EXPECT().Delete(gomock.Any(), "session").Return(nil)
	require.NoError(t, uc.Logout(context.Background(), "session"))
}

func TestMe_Success(t *testing.T) {
	uc := newTestUsecase(t)
	uc.sessions.EXPECT().GetByID(gomock.Any(), "session").Return(domain.Session{UserID: 7, ExpiresAt: time.Now().Add(time.Hour)}, nil)
	uc.users.EXPECT().GetByID(gomock.Any(), int32(7)).Return(domain.User{ID: 7}, nil)
	user, err := uc.Me(context.Background(), "session")
	require.NoError(t, err)
	require.Equal(t, int32(7), user.ID)
}

func TestMe_SessionNotFound(t *testing.T) {
	uc := newTestUsecase(t)
	uc.sessions.EXPECT().GetByID(gomock.Any(), "nope").Return(domain.Session{}, domain.ErrSessionNotFound)

	_, err := uc.Me(context.Background(), "nope")
	require.ErrorIs(t, err, domain.ErrSessionExpired)
}

func TestRegister_TxError(t *testing.T) {
	uc := newTestUsecase(t)
	commitErr := errors.New("commit failed")
	uc.tx.EXPECT().WithinTx(gomock.Any(), gomock.Any()).Return(commitErr)
	_, _, err := uc.Register(context.Background(), "a@b.ru", "Secret123", "ivan", "+79001234567")
	require.ErrorIs(t, err, commitErr)
}

func TestLogin_PasswordChangedBeforeSessionCreated(t *testing.T) {
	uc := newTestUsecase(t)
	uc.users.EXPECT().GetByEmail(gomock.Any(), "a@b.ru").Return(domain.User{ID: 7, PasswordHash: passwordHash(t, "Secret123")}, nil)
	expectTransaction(uc.tx)
	uc.users.EXPECT().GetPasswordHashForUpdate(gomock.Any(), int32(7)).Return("hash-after-change", nil)
	_, _, err := uc.Login(context.Background(), "a@b.ru", "Secret123")
	require.ErrorIs(t, err, domain.ErrInvalidLogin)
}
