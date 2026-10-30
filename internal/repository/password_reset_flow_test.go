//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
	"github.com/stretchr/testify/require"
)

type noopMailer struct{}

func (noopMailer) SendPasswordReset(context.Context, string, string) error { return nil }

func (noopMailer) SendEmailVerification(context.Context, string, string) error { return nil }

func sha256Hex(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func TestResetPassword_ConcurrentResetsWithDifferentTokens(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	users := NewUserRepository(pool)
	resets := NewPasswordResetRepository(pool)
	user, err := users.Create(ctx, "anna@goget.pw", "hash", "Анна", "+79990000001")
	require.NoError(t, err)
	uc := usecase.NewPasswordResetUsecase(users, NewSessionRepository(pool), resets, noopMailer{}, NewTransactor(pool), "http://front.test/reset-password")

	for i := range 60 {
		tokens := []string{"first-" + time.Now().String(), "second-" + time.Now().String()}
		for _, token := range tokens {
			require.NoError(t, resets.Create(ctx, sha256Hex(token), user.ID, time.Now().Add(time.Hour)))
		}

		errs := make([]error, len(tokens))
		var wg sync.WaitGroup
		for j, token := range tokens {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[j] = uc.ResetPassword(ctx, token, "NewSecret1")
			}()
		}
		wg.Wait()

		var succeeded int
		for _, err := range errs {
			if err == nil {
				succeeded++
				continue
			}
			require.ErrorIs(t, err, domain.ErrInvalidResetToken, "iteration %d", i)
		}
		require.Equal(t, 1, succeeded, "iteration %d", i)
	}
}

func TestResetPassword_LinkDiesAfterEmailOrPasswordChange(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	users := NewUserRepository(pool)
	sessions := NewSessionRepository(pool)
	resets := NewPasswordResetRepository(pool)
	tx := NewTransactor(pool)
	resetUC := usecase.NewPasswordResetUsecase(users, sessions, resets, noopMailer{}, tx, "http://front.test/reset-password")
	verifier := usecase.NewEmailVerificationUsecase(users, NewEmailVerificationRepository(pool), resets, noopMailer{}, tx, "http://front.test/verify-email")
	profileUC := usecase.NewProfileUsecase(users, sessions, resets, verifier, nil, tx)
	user, err := users.Create(ctx, "anna@goget.pw", "hash", "Анна", "+79990000001")
	require.NoError(t, err)

	require.NoError(t, resets.Create(ctx, sha256Hex("before-email"), user.ID, time.Now().Add(time.Hour)))
	email := "new@goget.pw"
	_, err = profileUC.UpdateProfile(ctx, user.ID, domain.ProfileUpdate{Email: &email})
	require.NoError(t, err)
	require.ErrorIs(t, resetUC.ResetPassword(ctx, "before-email", "NewSecret1"), domain.ErrInvalidResetToken)

	require.NoError(t, resets.Create(ctx, sha256Hex("before-password"), user.ID, time.Now().Add(time.Hour)))
	require.NoError(t, resetUC.ResetPassword(ctx, "before-password", "NewSecret1"))
	require.NoError(t, resets.Create(ctx, sha256Hex("after-reset"), user.ID, time.Now().Add(time.Hour)))
	require.NoError(t, profileUC.ChangePassword(ctx, user.ID, "session", "NewSecret1", "OtherSecret2"))
	require.ErrorIs(t, resetUC.ResetPassword(ctx, "after-reset", "ThirdSecret3"), domain.ErrInvalidResetToken)
}
