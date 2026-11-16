package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestCityUsecase_Search(t *testing.T) {
	repo := NewMockCityRepository(gomock.NewController(t))
	repo.EXPECT().Search(gomock.Any(), "Мос", citiesLimit).Return([]domain.City{{ID: 1, Name: "Москва"}}, nil)

	cities, err := NewCityUsecase(repo).Search(context.Background(), "  Мос ")
	require.NoError(t, err)
	require.Len(t, cities, 1)

}
