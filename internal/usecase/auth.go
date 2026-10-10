package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	sessionTTL       = 7 * 24 * time.Hour
	maxPasswordBytes = 72
	maxFirstNameLen  = 100
)

var (
	phoneFormat    = regexp.MustCompile(`^\+7\d{10}$`)
	nicknameFormat = regexp.MustCompile(`^[A-Za-z0-9_.]{3,32}$`)
)

type AuthUsecase struct {
	users    UserRepository
	sessions SessionRepository
}

func NewAuthUsecase(users UserRepository, sessions SessionRepository) *AuthUsecase {
	return &AuthUsecase{users: users, sessions: sessions}
}

func (uc *AuthUsecase) Register(ctx context.Context, email, password, firstName, nickname, phone string) (domain.User, domain.Session, error) {
	email = normalizeEmail(email)
	firstName = strings.TrimSpace(firstName)
	nickname = strings.TrimSpace(nickname)
	phone = normalizePhone(phone)

	if !isValidEmail(email) {
		return domain.User{}, domain.Session{}, domain.ErrInvalidEmail
	}
	if firstName == "" {
		return domain.User{}, domain.Session{}, domain.ErrMissingFirstName
	}
	if utf8.RuneCountInString(firstName) > maxFirstNameLen {
		return domain.User{}, domain.Session{}, domain.ErrFirstNameTooLong
	}
	if nickname == "" {
		return domain.User{}, domain.Session{}, domain.ErrMissingNickname
	}
	if !nicknameFormat.MatchString(nickname) {
		return domain.User{}, domain.Session{}, domain.ErrInvalidNickname
	}
	if !isValidPhone(phone) {
		return domain.User{}, domain.Session{}, domain.ErrInvalidPhone
	}
	if len(password) > maxPasswordBytes {
		return domain.User{}, domain.Session{}, domain.ErrPasswordTooLong
	}
	if !isStrongPassword(password) {
		return domain.User{}, domain.Session{}, domain.ErrWeakPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, domain.Session{}, err
	}

	user, err := uc.users.Create(ctx, email, string(hash), firstName, nickname, phone)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUserExists):
			return domain.User{}, domain.Session{}, domain.ErrEmailTaken
		case errors.Is(err, domain.ErrPhoneExists):
			return domain.User{}, domain.Session{}, domain.ErrPhoneTaken
		case errors.Is(err, domain.ErrNicknameExists):
			return domain.User{}, domain.Session{}, domain.ErrNicknameTaken
		default:
			return domain.User{}, domain.Session{}, err
		}
	}

	session, err := newSession(user.ID)
	if err != nil {
		return domain.User{}, domain.Session{}, err
	}

	if err := uc.sessions.Create(ctx, session.ID, session.UserID, session.ExpiresAt); err != nil {
		return domain.User{}, domain.Session{}, err
	}

	user.PasswordHash = ""

	return user, session, nil
}

func (uc *AuthUsecase) Login(ctx context.Context, login, password string) (domain.User, domain.Session, error) {
	login = strings.TrimSpace(login)

	var user domain.User
	var err error

	if strings.Contains(login, "@") {
		user, err = uc.users.GetByEmail(ctx, normalizeEmail(login))
	} else {
		user, err = uc.users.GetByPhone(ctx, normalizePhone(login))
	}

	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.User{}, domain.Session{}, domain.ErrInvalidLogin
		}
		return domain.User{}, domain.Session{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return domain.User{}, domain.Session{}, domain.ErrInvalidLogin
	}

	session, err := newSession(user.ID)
	if err != nil {
		return domain.User{}, domain.Session{}, err
	}

	if err := uc.sessions.Create(ctx, session.ID, session.UserID, session.ExpiresAt); err != nil {
		return domain.User{}, domain.Session{}, err
	}

	user.PasswordHash = ""

	return user, session, nil
}

func (uc *AuthUsecase) Logout(ctx context.Context, sessionID string) error {
	return uc.sessions.Delete(ctx, sessionID)
}

func (uc *AuthUsecase) Me(ctx context.Context, sessionID string) (domain.User, error) {
	session, err := uc.sessions.GetByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return domain.User{}, domain.ErrSessionExpired
		}
		return domain.User{}, err
	}

	if time.Now().After(session.ExpiresAt) {
		if err := uc.sessions.Delete(ctx, sessionID); err != nil {
			return domain.User{}, err
		}
		return domain.User{}, domain.ErrSessionExpired
	}

	return uc.users.GetByID(ctx, session.UserID)
}

func newSession(userID int32) (domain.Session, error) {
	token, err := generateToken()
	if err != nil {
		return domain.Session{}, err
	}

	return domain.Session{
		ID:        token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionTTL),
	}, nil
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

func isValidEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizePhone(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}
	d := digits.String()

	switch {
	case len(d) == 11 && (d[0] == '7' || d[0] == '8'):
		return "+7" + d[1:]
	case len(d) == 10:
		return "+7" + d
	}
	return d
}

func isValidPhone(phone string) bool {
	return phoneFormat.MatchString(phone)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
