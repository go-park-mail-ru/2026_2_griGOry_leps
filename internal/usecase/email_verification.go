package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	verifyTokenTTL    = 30 * time.Minute
	verifyCooldown    = time.Minute
	verifyMailTimeout = 30 * time.Second
)

type EmailVerificationUsecase struct {
	users         UserRepository
	verifications EmailVerificationRepository
	resets        PasswordResetRepository
	mailer        Mailer
	tx            Transactor
	verifyURL     string
}

func NewEmailVerificationUsecase(users UserRepository, verifications EmailVerificationRepository, resets PasswordResetRepository,
	mailer Mailer, tx Transactor, verifyURL string) *EmailVerificationUsecase {
	return &EmailVerificationUsecase{
		users: users, verifications: verifications, resets: resets, mailer: mailer, tx: tx, verifyURL: verifyURL,
	}
}

// IssueLink отзывает прежние ссылки и создаёт новую. Письмо уходит, когда вызовут send — после коммита.
func (uc *EmailVerificationUsecase) IssueLink(ctx context.Context, userID int32, email string) (func(), error) {
	now := time.Now()
	last, err := uc.verifications.LastCreatedAt(ctx, userID)
	if err != nil {
		return nil, err
	}
	if next := last.Add(verifyCooldown); now.Before(next) {
		return nil, &domain.CooldownError{Wait: next.Sub(now)}
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}
	if err := uc.verifications.UseAll(ctx, userID); err != nil {
		return nil, err
	}
	if err := uc.verifications.Create(ctx, hashToken(token), userID, email, now.Add(verifyTokenTTL)); err != nil {
		return nil, err
	}

	link := uc.verifyURL + "?token=" + token
	return func() { go uc.sendMail(context.WithoutCancel(ctx), email, link) }, nil
}

func (uc *EmailVerificationUsecase) RevokeLinks(ctx context.Context, userID int32) error {
	return uc.verifications.UseAll(ctx, userID)
}

func (uc *EmailVerificationUsecase) sendMail(ctx context.Context, to, link string) {
	ctx, cancel := context.WithTimeout(ctx, verifyMailTimeout)
	defer cancel()

	if err := uc.mailer.SendEmailVerification(ctx, to, link); err != nil {
		slog.ErrorContext(ctx, "send email verification mail", "err", err)
	}
}

func unconfirmedEmail(user domain.User) string {
	if user.PendingEmail != "" {
		return user.PendingEmail
	}
	if !user.EmailVerified {
		return user.Email
	}
	return ""
}

func (uc *EmailVerificationUsecase) Resend(ctx context.Context, userID int32) (string, time.Duration, error) {
	var email string
	var send func()
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.users.LockByID(ctx, userID); err != nil {
			return err
		}
		user, err := uc.users.GetByID(ctx, userID)
		if err != nil {
			return err
		}
		email = unconfirmedEmail(user)
		if email == "" {
			return domain.ErrEmailAlreadyVerified
		}

		send, err = uc.IssueLink(ctx, userID, email)
		return err
	})
	var cooldown *domain.CooldownError
	if errors.As(err, &cooldown) {
		return email, cooldown.Wait, nil
	}
	if err != nil {
		return email, 0, err
	}

	send()
	return email, 0, nil
}

func (uc *EmailVerificationUsecase) ResendByToken(ctx context.Context, token string) (string, time.Duration, error) {
	if token == "" {
		return "", 0, domain.ErrInvalidVerifyToken
	}
	v, err := uc.verifications.Get(ctx, hashToken(token))
	if err != nil {
		return "", 0, err
	}
	user, err := uc.users.GetByID(ctx, v.UserID)
	if err != nil {
		return "", 0, err
	}
	if v.Email != unconfirmedEmail(user) {
		return "", 0, domain.ErrInvalidVerifyToken
	}
	return uc.Resend(ctx, v.UserID)
}

func (uc *EmailVerificationUsecase) Verify(ctx context.Context, token string) (domain.EmailVerifyResult, error) {
	if token == "" {
		return domain.EmailVerifyResult{}, domain.ErrInvalidVerifyToken
	}

	tokenHash := hashToken(token)
	v, err := uc.verifications.Get(ctx, tokenHash)
	if err != nil {
		return domain.EmailVerifyResult{}, err
	}

	var result domain.EmailVerifyResult
	err = uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.users.LockByID(ctx, v.UserID); err != nil {
			return err
		}
		locked, err := uc.verifications.GetForUpdate(ctx, tokenHash)
		if err != nil {
			return err
		}
		user, err := uc.users.GetByID(ctx, v.UserID)
		if err != nil {
			return err
		}

		if !locked.Valid(time.Now()) {
			if locked.Email == user.Email && user.EmailVerified {
				return domain.ErrEmailAlreadyVerified
			}
			if locked.Email == unconfirmedEmail(user) {
				result.Email = locked.Email
			}
			return domain.ErrInvalidVerifyToken
		}

		switch locked.Email {
		case user.PendingEmail:
			if err := uc.users.ConfirmPendingEmail(ctx, user.ID); err != nil {
				return err
			}
			if err := uc.resets.UseAll(ctx, user.ID); err != nil {
				return err
			}
			result.Changed = true
		case user.Email:
			if user.EmailVerified {
				return domain.ErrEmailAlreadyVerified
			}
			if err := uc.users.SetEmailVerified(ctx, user.ID); err != nil {
				return err
			}
		default:
			return domain.ErrInvalidVerifyToken
		}

		result.Email = locked.Email
		return uc.verifications.UseAll(ctx, user.ID)
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

func (uc *EmailVerificationUsecase) CancelEmailChange(ctx context.Context, userID int32) error {
	return uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.users.LockByID(ctx, userID); err != nil {
			return err
		}
		user, err := uc.users.GetByID(ctx, userID)
		if err != nil {
			return err
		}
		if user.PendingEmail == "" {
			return domain.ErrNoPendingEmail
		}
		if err := uc.users.SetPendingEmail(ctx, userID, nil); err != nil {
			return err
		}
		return uc.RevokeLinks(ctx, userID)
	})
}
