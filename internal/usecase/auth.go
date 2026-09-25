package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
)

var (
	ErrInvalidEmail     = errors.New("invalid email")
	ErrMissingFirstName = errors.New("first name is required")
	ErrMissingNickname  = errors.New("nickname is required")
	ErrInvalidPhone     = errors.New("invalid phone number")
	ErrWeakPassword     = errors.New("password must be at least 8 characters and contain uppercase, lowercase letters and a digit")
	ErrEmailTaken       = errors.New("email already registered")
	ErrPhoneTaken       = errors.New("phone already registered")
	ErrInvalidLogin     = errors.New("invalid login or password")
	ErrSessionExpired   = errors.New("session expired")
)

const sessionTTL = 7 * 24 * time.Hour

type AuthUsecase struct {
	users    *repository.UserRepository
	sessions *repository.SessionRepository
}

func NewAuthUsecase(users *repository.UserRepository, sessions *repository.SessionRepository) *AuthUsecase {
	return &AuthUsecase{users: users, sessions: sessions}
}

func (uc *AuthUsecase) Register(ctx context.Context, email, password, firstName, nickname, phone string) (domain.User, error) {
	email = normalizeEmail(email)
	firstName = strings.TrimSpace(firstName)
	nickname = strings.TrimSpace(nickname)
	phone = normalizePhone(phone)

	if _, err := mail.ParseAddress(email); err != nil {
		return domain.User{}, ErrInvalidEmail
	}
	if firstName == "" {
		return domain.User{}, ErrMissingFirstName
	}
	if nickname == "" {
		return domain.User{}, ErrMissingNickname
	}
	if !isValidPhone(phone) {
		return domain.User{}, ErrInvalidPhone
	}
	if !isStrongPassword(password) {
		return domain.User{}, ErrWeakPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}

	user, err := uc.users.Create(ctx, email, string(hash), firstName, nickname, phone)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUserExists):
			return domain.User{}, ErrEmailTaken
		case errors.Is(err, repository.ErrPhoneExists):
			return domain.User{}, ErrPhoneTaken
		default:
			return domain.User{}, err
		}
	}

	return user, nil
}

func (uc *AuthUsecase) Login(ctx context.Context, login, password string) (domain.Session, error) {
	login = strings.TrimSpace(login)

	var user domain.User
	var err error

	if strings.Contains(login, "@") {
		user, err = uc.users.GetByEmail(ctx, normalizeEmail(login))
	} else {
		user, err = uc.users.GetByPhone(ctx, normalizePhone(login))
	}

	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return domain.Session{}, ErrInvalidLogin
		}
		return domain.Session{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return domain.Session{}, ErrInvalidLogin
	}

	token, err := generateToken()
	if err != nil {
		return domain.Session{}, err
	}

	session := domain.Session{
		ID:        token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(sessionTTL),
	}

	if err := uc.sessions.Create(ctx, session.ID, session.UserID, session.ExpiresAt); err != nil {
		return domain.Session{}, err
	}

	return session, nil
}

func (uc *AuthUsecase) Logout(ctx context.Context, sessionID string) error {
	return uc.sessions.Delete(ctx, sessionID)
}

func (uc *AuthUsecase) Me(ctx context.Context, sessionID string) (domain.User, error) {
	session, err := uc.sessions.GetByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return domain.User{}, ErrSessionExpired
		}
		return domain.User{}, err
	}

	if time.Now().After(session.ExpiresAt) {
		return domain.User{}, ErrSessionExpired
	}

	return uc.users.GetByID(ctx, session.UserID)
}

func isStrongPassword(password string) bool {
	if utf8.RuneCountInString(password) < 8 {
		return false
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	return hasUpper && hasLower && hasDigit
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizePhone(phone string) string {
	hasPlus := strings.HasPrefix(strings.TrimSpace(phone), "+")

	var digits strings.Builder
	for _, r := range phone {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}
	d := digits.String()

	if len(d) == 11 && (d[0] == '8' || d[0] == '7') {
		return "+7" + d[1:]
	}
	if hasPlus {
		return "+" + d
	}
	return d
}

func isValidPhone(phone string) bool {
	digits := 0
	for _, r := range phone {
		if unicode.IsDigit(r) {
			digits++
		}
	}
	return digits >= 10
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
