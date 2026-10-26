package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	resetTokenTTL    = 30 * time.Minute
	resetCooldown    = time.Minute
	resetMailTimeout = 30 * time.Second
)

type PasswordResetUsecase struct {
	users    UserRepository
	sessions SessionRepository
	resets   PasswordResetRepository
	mailer   Mailer
	tx       Transactor
	resetURL string
}

func NewPasswordResetUsecase(users UserRepository, sessions SessionRepository, resets PasswordResetRepository,
	mailer Mailer, tx Transactor, resetURL string) *PasswordResetUsecase {
	return &PasswordResetUsecase{users: users, sessions: sessions, resets: resets, mailer: mailer, tx: tx, resetURL: resetURL}
}

func (uc *PasswordResetUsecase) RequestReset(ctx context.Context, login string) error {
	user, err := uc.findUser(ctx, strings.TrimSpace(login))
	if errors.Is(err, domain.ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	token, err := generateToken()
	if err != nil {
		return err
	}

	now := time.Now()
	var created bool
	err = uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.users.LockByID(ctx, user.ID); err != nil {
			return err
		}

		current, err := uc.users.GetByID(ctx, user.ID)
		if err != nil {
			return err
		}
		if current.Email != user.Email {
			return nil
		}

		recent, err := uc.resets.HasCreatedSince(ctx, user.ID, now.Add(-resetCooldown))
		if err != nil {
			return err
		}
		if recent {
			return nil
		}

		created = true
		return uc.resets.Create(ctx, hashToken(token), user.ID, now.Add(resetTokenTTL))
	})
	if err != nil {
		return err
	}
	if !created {
		return nil
	}

	go uc.sendResetMail(context.WithoutCancel(ctx), user.Email, uc.resetURL+"?token="+token)
	return nil
}

func (uc *PasswordResetUsecase) findUser(ctx context.Context, login string) (domain.User, error) {
	if strings.Contains(login, "@") {
		email := normalizeEmail(login)
		if !isValidEmail(email) {
			return domain.User{}, domain.ErrInvalidContact
		}
		return uc.users.GetByEmail(ctx, email)
	}

	phone := normalizePhone(login)
	if !isValidPhone(phone) {
		return domain.User{}, domain.ErrInvalidContact
	}
	return uc.users.GetByPhone(ctx, phone)
}

func (uc *PasswordResetUsecase) sendResetMail(ctx context.Context, to, link string) {
	ctx, cancel := context.WithTimeout(ctx, resetMailTimeout)
	defer cancel()

	if err := uc.mailer.SendPasswordReset(ctx, to, link); err != nil {
		slog.ErrorContext(ctx, "send password reset mail", "err", err)
	}
}

func (uc *PasswordResetUsecase) CheckToken(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", domain.ErrInvalidResetToken
	}

	reset, err := uc.resets.Get(ctx, hashToken(token))
	if err != nil {
		return "", err
	}
	if !reset.Valid(time.Now()) {
		return "", domain.ErrInvalidResetToken
	}

	user, err := uc.users.GetByID(ctx, reset.UserID)
	if err != nil {
		return "", err
	}
	return user.Email, nil
}

func (uc *PasswordResetUsecase) ResetPassword(ctx context.Context, token, password string) error {
	if token == "" {
		return domain.ErrInvalidResetToken
	}
	if err := checkPassword(password); err != nil {
		return err
	}

	tokenHash := hashToken(token)
	reset, err := uc.resets.Get(ctx, tokenHash)
	if err != nil {
		return err
	}
	if !reset.Valid(time.Now()) {
		return domain.ErrInvalidResetToken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.users.LockByID(ctx, reset.UserID); err != nil {
			return err
		}

		locked, err := uc.resets.GetForUpdate(ctx, tokenHash)
		if err != nil {
			return err
		}
		if !locked.Valid(time.Now()) {
			return domain.ErrInvalidResetToken
		}

		if err := uc.users.UpdatePassword(ctx, reset.UserID, string(hash)); err != nil {
			return err
		}
		if err := uc.resets.UseAll(ctx, reset.UserID); err != nil {
			return err
		}
		return uc.sessions.DeleteAll(ctx, reset.UserID)
	})
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
