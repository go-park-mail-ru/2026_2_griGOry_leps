package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
)

var (
	ErrInvalidEmail   = errors.New("invalid email")
	ErrWeakPassword   = errors.New("password must be at least 8 characters")
	ErrEmailTaken     = errors.New("email already registered")
	ErrInvalidLogin   = errors.New("invalid email or password")
	ErrSessionExpired = errors.New("session expired")
)

const sessionTTL = 7 * 24 * time.Hour

type AuthUsecase struct {
	users    *repository.UserRepository
	sessions *repository.SessionRepository
}

func NewAuthUsecase(users *repository.UserRepository, sessions *repository.SessionRepository) *AuthUsecase {
	return &AuthUsecase{users: users, sessions: sessions}
}

func (uc *AuthUsecase) Register(ctx context.Context, email, password string) (domain.User, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return domain.User{}, ErrInvalidEmail
	}
	if len(password) < 8 {
		return domain.User{}, ErrWeakPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}

	user, err := uc.users.Create(ctx, email, string(hash))
	if err != nil {
		if errors.Is(err, repository.ErrUserExists) {
			return domain.User{}, ErrEmailTaken
		}
		return domain.User{}, err
	}

	return user, nil
}

func (uc *AuthUsecase) Login(ctx context.Context, email, password string) (domain.Session, error) {
	user, err := uc.users.GetByEmail(ctx, email)
	if err != nil {
		return domain.Session{}, ErrInvalidLogin
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
		return domain.User{}, ErrSessionExpired
	}

	if time.Now().After(session.ExpiresAt) {
		return domain.User{}, ErrSessionExpired
	}

	return uc.users.GetByID(ctx, session.UserID)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}