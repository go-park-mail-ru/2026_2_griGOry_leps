package usecase

import (
	"context"
	"strings"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const citiesLimit = 10

type CityUsecase struct {
	cities CityRepository
}

func NewCityUsecase(cities CityRepository) *CityUsecase {
	return &CityUsecase{cities: cities}
}

func (uc *CityUsecase) Search(ctx context.Context, query string) ([]domain.City, error) {
	return uc.cities.Search(ctx, strings.TrimSpace(query), citiesLimit)
}
