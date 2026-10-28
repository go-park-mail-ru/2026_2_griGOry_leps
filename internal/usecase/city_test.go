package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type cityRepoStub struct {
	prefix string
	limit  int
}

func (s *cityRepoStub) Search(_ context.Context, prefix string, limit int) ([]domain.City, error) {
	s.prefix, s.limit = prefix, limit
	return []domain.City{{ID: 1, Name: "Москва"}}, nil
}

func TestCityUsecase_Search(t *testing.T) {
	repo := &cityRepoStub{}

	cities, err := NewCityUsecase(repo).Search(context.Background(), "  Мос ")
	require.NoError(t, err)
	require.Len(t, cities, 1)
	require.Equal(t, "Мос", repo.prefix)
	require.Equal(t, citiesLimit, repo.limit)
}
