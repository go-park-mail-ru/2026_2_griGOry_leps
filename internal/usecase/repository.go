package usecase

import (
	"context"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, email, passwordHash, firstName, nickname, phone string) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetByPhone(ctx context.Context, phone string) (domain.User, error)
	GetByID(ctx context.Context, id int32) (domain.User, error)
}

type SessionRepository interface {
	Create(ctx context.Context, id string, userID int32, expiresAt time.Time) error
	GetByID(ctx context.Context, id string) (domain.Session, error)
	Delete(ctx context.Context, id string) error
}

type AdRepository interface {
	List(ctx context.Context, f domain.AdFilter) ([]domain.Ad, int, error)
}
