package usecase

import (
	"context"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, email, passwordHash, firstName, phone string) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetByPhone(ctx context.Context, phone string) (domain.User, error)
	GetByID(ctx context.Context, id int32) (domain.User, error)
	GetProfile(ctx context.Context, id int32) (domain.Profile, error)
	UpdateProfile(ctx context.Context, id int32, upd domain.ProfileUpdate) error
	UpdatePassword(ctx context.Context, id int32, passwordHash string) error
	GetAvatarForUpdate(ctx context.Context, id int32) (string, error)
	UpdateAvatar(ctx context.Context, id int32, avatar string) error
}

type SessionRepository interface {
	Create(ctx context.Context, id string, userID int32, expiresAt time.Time) error
	GetByID(ctx context.Context, id string) (domain.Session, error)
	Delete(ctx context.Context, id string) error
	DeleteOthers(ctx context.Context, userID int32, keepID string) error
}

type FileStorage interface {
	Save(ctx context.Context, name string, data []byte) error
	Delete(ctx context.Context, name string) error
}

type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type AdRepository interface {
	List(ctx context.Context, f domain.AdFilter) ([]domain.Ad, int, error)
}
