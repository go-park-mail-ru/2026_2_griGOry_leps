package usecase

import (
	"context"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type FavoritesRepository interface {
	List(ctx context.Context, userID int32) ([]domain.Ad, error)
	Add(ctx context.Context, userID, listingID int32) error
	Remove(ctx context.Context, userID, listingID int32) error
}

type FavoritesUsecase struct {
	favorites FavoritesRepository
}

func NewFavoritesUsecase(favorites FavoritesRepository) *FavoritesUsecase {
	return &FavoritesUsecase{favorites: favorites}
}

func (uc *FavoritesUsecase) List(ctx context.Context, userID int32) ([]domain.Ad, error) {
	return uc.favorites.List(ctx, userID)
}

func (uc *FavoritesUsecase) Add(ctx context.Context, userID, listingID int32) error {
	return uc.favorites.Add(ctx, userID, listingID)
}

func (uc *FavoritesUsecase) Remove(ctx context.Context, userID, listingID int32) error {
	return uc.favorites.Remove(ctx, userID, listingID)
}
