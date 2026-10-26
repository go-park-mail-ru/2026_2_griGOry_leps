package usecase

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	maxAboutLength  = 500
	maxAvatarPixels = 4096 * 4096
)

type ProfileUsecase struct {
	users    UserRepository
	sessions SessionRepository
	resets   PasswordResetRepository
	avatars  FileStorage
	tx       Transactor
}

func NewProfileUsecase(users UserRepository, sessions SessionRepository, resets PasswordResetRepository,
	avatars FileStorage, tx Transactor) *ProfileUsecase {
	return &ProfileUsecase{users: users, sessions: sessions, resets: resets, avatars: avatars, tx: tx}
}

func (uc *ProfileUsecase) Profile(ctx context.Context, userID int32) (domain.Profile, error) {
	return uc.users.GetProfile(ctx, userID)
}

func (uc *ProfileUsecase) UpdateProfile(ctx context.Context, userID int32, upd domain.ProfileUpdate) (domain.Profile, error) {
	if err := normalizeProfileUpdate(&upd); err != nil {
		return domain.Profile{}, err
	}

	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.users.LockByID(ctx, userID); err != nil {
			return err
		}

		before, err := uc.users.GetByID(ctx, userID)
		if err != nil {
			return err
		}
		if err := uc.users.UpdateProfile(ctx, userID, upd); err != nil {
			return err
		}

		if upd.Email != nil && *upd.Email != before.Email {
			return uc.resets.UseAll(ctx, userID)
		}
		return nil
	})
	if err != nil {
		return domain.Profile{}, err
	}
	return uc.users.GetProfile(ctx, userID)
}

func normalizeProfileUpdate(upd *domain.ProfileUpdate) error {
	if upd.FirstName != nil {
		name := normalizeFirstName(*upd.FirstName)
		if err := checkFirstName(name); err != nil {
			return err
		}
		upd.FirstName = &name
	}

	if upd.Phone != nil {
		phone := normalizePhone(*upd.Phone)
		if !isValidPhone(phone) {
			return domain.ErrInvalidPhone
		}
		upd.Phone = &phone
	}

	if upd.Email != nil {
		email := normalizeEmail(*upd.Email)
		if !isValidEmail(email) {
			return domain.ErrInvalidEmail
		}
		upd.Email = &email
	}

	if upd.CityID != nil && *upd.CityID <= 0 {
		return domain.ErrInvalidCity
	}

	if upd.About != nil {
		about := strings.TrimSpace(*upd.About)
		if utf8.RuneCountInString(about) > maxAboutLength {
			return domain.ErrAboutTooLong
		}
		upd.About = &about
	}

	return nil
}

func (uc *ProfileUsecase) ChangePassword(ctx context.Context, userID int32, sessionID, current, next string) error {
	user, err := uc.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)); err != nil {
		return domain.ErrWrongPassword
	}
	if err := checkPassword(next); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		actual, err := uc.users.GetPasswordHashForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if actual != user.PasswordHash {
			return domain.ErrWrongPassword
		}
		if err := uc.users.UpdatePassword(ctx, userID, string(hash)); err != nil {
			return err
		}
		if err := uc.resets.UseAll(ctx, userID); err != nil {
			return err
		}
		return uc.sessions.DeleteOthers(ctx, userID, sessionID)
	})
}

func (uc *ProfileUsecase) UploadAvatar(ctx context.Context, userID int32, data []byte) (string, error) {
	ext, err := imageExt(data)
	if err != nil {
		return "", err
	}

	token, err := generateToken()
	if err != nil {
		return "", err
	}
	name := token[:32] + ext

	if err := uc.avatars.Save(ctx, name, data); err != nil {
		return "", err
	}

	var old string
	err = uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		old, err = uc.users.GetAvatarForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		return uc.users.UpdateAvatar(ctx, userID, name)
	})
	if err != nil {
		uc.removeAvatar(ctx, name)
		return "", err
	}

	if old != "" {
		uc.removeAvatar(ctx, old)
	}
	return name, nil
}

func (uc *ProfileUsecase) removeAvatar(ctx context.Context, name string) {
	if err := uc.avatars.Delete(ctx, name); err != nil {
		slog.WarnContext(ctx, "delete avatar", "file", name, "err", err)
	}
}

func imageExt(data []byte) (string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	switch {
	case errors.Is(err, image.ErrFormat):
		return "", domain.ErrUnsupportedImage
	case err != nil:
		return "", domain.ErrInvalidImage
	case format != "jpeg" && format != "png":
		return "", domain.ErrUnsupportedImage
	}

	if cfg.Width*cfg.Height > maxAvatarPixels {
		return "", domain.ErrImageTooLarge
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return "", domain.ErrInvalidImage
	}

	if format == "png" {
		return ".png", nil
	}
	return ".jpg", nil
}
