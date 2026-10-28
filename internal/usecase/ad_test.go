package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type adRepoStub struct {
	items    []domain.Ad
	filter   domain.AdFilter
	err      error
	myFilter domain.MyAdFilter
	counts   map[string]int
}

func (s *adRepoStub) ListByUser(_ context.Context, f domain.MyAdFilter) ([]domain.Ad, map[string]int, error) {
	s.myFilter = f
	return s.items, s.counts, s.err
}

func (s *adRepoStub) List(
	ctx context.Context,
	filter domain.AdFilter,
) ([]domain.Ad, int, error) {
	s.filter = filter
	return s.items, len(s.items), s.err
}

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
				require.ErrorIs(t, err, domain.ErrInvalidPrice)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAdUsecase_List_DefaultLimit(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{})
	page, err := uc.List(context.Background(), domain.ListAdsParams{})
	require.NoError(t, err)
	require.Equal(t, defaultAdsLimit, page.Limit)
}

func TestAdUsecase_List_LimitCappedAt60(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{})
	page, err := uc.List(context.Background(), domain.ListAdsParams{Limit: 1000})
	require.NoError(t, err)
	require.Equal(t, maxAdsLimit, page.Limit)
}

func TestAdUsecase_List_InvalidSort(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{})
	_, err := uc.List(context.Background(), domain.ListAdsParams{Sort: "unknown"})
	require.ErrorIs(t, err, domain.ErrInvalidSort)
}

func TestAdUsecase_List_DefaultSort(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{})
	_, err := uc.List(context.Background(), domain.ListAdsParams{Sort: ""})
	require.NoError(t, err)
}

func TestAdUsecase_List_InvalidPriceMin(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{})
	_, err := uc.List(context.Background(), domain.ListAdsParams{PriceMin: "abc"})
	require.ErrorIs(t, err, domain.ErrInvalidPrice)
}

func TestAdUsecase_List_InvalidPriceMax(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{})
	_, err := uc.List(context.Background(), domain.ListAdsParams{PriceMax: "abc"})
	require.ErrorIs(t, err, domain.ErrInvalidPrice)
}

func TestAdUsecase_List_PriceRangeInvalid(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{})
	_, err := uc.List(context.Background(), domain.ListAdsParams{
		PriceMin: "5000", PriceMax: "1000",
	})
	require.ErrorIs(t, err, domain.ErrInvalidPriceRange)
}

func TestAdUsecase_List_Success(t *testing.T) {
	price := int64(100000)
	ads := []domain.Ad{
		{
			ID: 1, Title: "Тест", Price: &price, CategoryID: 1, City: "Москва",
			Status: domain.AdStatusActive, CreatedAt: time.Now(),
		},
	}
	uc := NewAdUsecase(&adRepoStub{items: ads})

	page, err := uc.List(context.Background(), domain.ListAdsParams{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	require.Equal(t, 10, page.Limit)
	require.Equal(t, 0, page.Offset)
}

func TestAdUsecase_ListMine_CountsAllStatuses(t *testing.T) {
	repo := &adRepoStub{counts: map[string]int{domain.AdStatusActive: 3, domain.AdStatusDraft: 1}}

	page, err := NewAdUsecase(repo).ListMine(context.Background(), domain.MyAdFilter{UserID: 7, Limit: 100})
	require.NoError(t, err)
	require.Equal(t, map[string]int{
		"all": 4, "draft": 1, "moderation": 0, "rejected": 0, "active": 3, "closed": 0, "sold": 0,
	}, page.Counts)
	require.Equal(t, 4, page.Total)
	require.Equal(t, maxAdsLimit, page.Limit)
	require.Equal(t, int32(7), repo.myFilter.UserID)
}

func TestAdUsecase_ListMine_TotalByStatus(t *testing.T) {
	repo := &adRepoStub{counts: map[string]int{domain.AdStatusActive: 3, domain.AdStatusDraft: 1}}

	page, err := NewAdUsecase(repo).ListMine(context.Background(), domain.MyAdFilter{Status: "draft"})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Equal(t, defaultAdsLimit, page.Limit)
}

func TestAdUsecase_ListMine_InvalidStatus(t *testing.T) {
	repo := &adRepoStub{}
	_, err := NewAdUsecase(repo).ListMine(context.Background(), domain.MyAdFilter{Status: "deleted"})
	require.ErrorIs(t, err, domain.ErrInvalidStatus)
	require.Zero(t, repo.myFilter)
}
