package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
)

func TestParsePrice(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    int64
		wantErr bool
	}{
		{"integer", "1500", 150000, false},
		{"one decimal", "1500.5", 150050, false},
		{"two decimals", "1500.50", 150050, false},
		{"zero", "0", 0, false},
		{"invalid letters", "abc", 0, true},
		{"empty", "", 0, true},
		{"three decimals", "1500.123", 0, true},
		{"negative", "-100", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePrice(tt.in)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidPrice)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAdUsecase_List_DefaultLimit(t *testing.T) {
	uc := NewAdUsecase(repository.NewAdRepository(nil))
	page, err := uc.List(context.Background(), ListAdsParams{})
	require.NoError(t, err)
	require.Equal(t, defaultAdsLimit, page.Limit)
}

func TestAdUsecase_List_LimitCappedAt60(t *testing.T) {
	uc := NewAdUsecase(repository.NewAdRepository(nil))
	page, err := uc.List(context.Background(), ListAdsParams{Limit: 1000})
	require.NoError(t, err)
	require.Equal(t, maxAdsLimit, page.Limit)
}

func TestAdUsecase_List_InvalidSort(t *testing.T) {
	uc := NewAdUsecase(repository.NewAdRepository(nil))
	_, err := uc.List(context.Background(), ListAdsParams{Sort: "unknown"})
	require.ErrorIs(t, err, ErrInvalidSort)
}

func TestAdUsecase_List_DefaultSort(t *testing.T) {
	uc := NewAdUsecase(repository.NewAdRepository(nil))
	_, err := uc.List(context.Background(), ListAdsParams{Sort: ""})
	require.NoError(t, err)
}

func TestAdUsecase_List_InvalidPriceMin(t *testing.T) {
	uc := NewAdUsecase(repository.NewAdRepository(nil))
	_, err := uc.List(context.Background(), ListAdsParams{PriceMin: "abc"})
	require.ErrorIs(t, err, ErrInvalidPrice)
}

func TestAdUsecase_List_InvalidPriceMax(t *testing.T) {
	uc := NewAdUsecase(repository.NewAdRepository(nil))
	_, err := uc.List(context.Background(), ListAdsParams{PriceMax: "abc"})
	require.ErrorIs(t, err, ErrInvalidPrice)
}

func TestAdUsecase_List_PriceRangeInvalid(t *testing.T) {
	uc := NewAdUsecase(repository.NewAdRepository(nil))
	_, err := uc.List(context.Background(), ListAdsParams{
		PriceMin: "5000", PriceMax: "1000",
	})
	require.ErrorIs(t, err, ErrInvalidPriceRange)
}

func TestAdUsecase_List_Success(t *testing.T) {
	ads := []domain.Ad{
		{
			ID: 1, Title: "Тест", Price: 100000, CategoryID: 1, City: "Москва",
			Status: domain.AdStatusActive, CreatedAt: time.Now(),
		},
	}
	uc := NewAdUsecase(repository.NewAdRepository(ads))

	page, err := uc.List(context.Background(), ListAdsParams{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	require.Equal(t, 10, page.Limit)
	require.Equal(t, 0, page.Offset)
}
