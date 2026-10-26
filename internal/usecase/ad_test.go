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
	ownerID  int32
	status   string
	closed   string
	deleted  bool
	viewerID int32
}

func (s *adRepoStub) ListByUser(_ context.Context, f domain.MyAdFilter) ([]domain.Ad, map[string]int, error) {
	s.myFilter = f
	return s.items, s.counts, s.err
}

func (s *adRepoStub) GetForUpdate(context.Context, int32) (int32, string, error) {
	return s.ownerID, s.status, s.err
}

func (s *adRepoStub) Close(_ context.Context, _ int32, reason string) error {
	s.closed = reason
	return nil
}

func (s *adRepoStub) Delete(context.Context, int32) error {
	s.deleted = true
	return nil
}

func (s *adRepoStub) GetListing(_ context.Context, id, viewerID int32) (domain.Listing, error) {
	s.viewerID = viewerID
	return domain.Listing{Ad: domain.Ad{ID: id, Status: domain.AdStatusClosed, CloseReason: s.closed}}, nil
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
	uc := NewAdUsecase(&adRepoStub{}, txFake{})
	page, err := uc.List(context.Background(), domain.ListAdsParams{})
	require.NoError(t, err)
	require.Equal(t, defaultAdsLimit, page.Limit)
}

func TestAdUsecase_List_LimitCappedAt60(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{}, txFake{})
	page, err := uc.List(context.Background(), domain.ListAdsParams{Limit: 1000})
	require.NoError(t, err)
	require.Equal(t, maxAdsLimit, page.Limit)
}

func TestAdUsecase_List_InvalidSort(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{}, txFake{})
	_, err := uc.List(context.Background(), domain.ListAdsParams{Sort: "unknown"})
	require.ErrorIs(t, err, domain.ErrInvalidSort)
}

func TestAdUsecase_List_DefaultSort(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{}, txFake{})
	_, err := uc.List(context.Background(), domain.ListAdsParams{Sort: ""})
	require.NoError(t, err)
}

func TestAdUsecase_List_InvalidPriceMin(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{}, txFake{})
	_, err := uc.List(context.Background(), domain.ListAdsParams{PriceMin: "abc"})
	require.ErrorIs(t, err, domain.ErrInvalidPrice)
}

func TestAdUsecase_List_InvalidPriceMax(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{}, txFake{})
	_, err := uc.List(context.Background(), domain.ListAdsParams{PriceMax: "abc"})
	require.ErrorIs(t, err, domain.ErrInvalidPrice)
}

func TestAdUsecase_List_PriceRangeInvalid(t *testing.T) {
	uc := NewAdUsecase(&adRepoStub{}, txFake{})
	_, err := uc.List(context.Background(), domain.ListAdsParams{
		PriceMin: "5000", PriceMax: "1000",
	})
	require.ErrorIs(t, err, domain.ErrInvalidPriceRange)
}

func TestAdUsecase_List_Success(t *testing.T) {
	ads := []domain.Ad{
		{
			ID: 1, Title: "Тест", Price: 100000, CategoryID: 1, City: "Москва",
			Status: domain.AdStatusActive, CreatedAt: time.Now(),
		},
	}
	uc := NewAdUsecase(&adRepoStub{items: ads}, txFake{})

	page, err := uc.List(context.Background(), domain.ListAdsParams{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	require.Equal(t, 10, page.Limit)
	require.Equal(t, 0, page.Offset)
}

func TestAdUsecase_ListMine_CountsAllStatuses(t *testing.T) {
	repo := &adRepoStub{counts: map[string]int{domain.AdStatusActive: 3, domain.AdStatusDraft: 1}}
	uc := NewAdUsecase(repo, txFake{})

	page, err := uc.ListMine(context.Background(), domain.MyAdFilter{UserID: 7, Limit: 100})
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

	page, err := NewAdUsecase(repo, txFake{}).ListMine(context.Background(), domain.MyAdFilter{Status: "draft"})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Equal(t, defaultAdsLimit, page.Limit)
}

func TestAdUsecase_ListMine_InvalidStatus(t *testing.T) {
	_, err := NewAdUsecase(&adRepoStub{}, txFake{}).ListMine(context.Background(), domain.MyAdFilter{Status: "deleted"})
	require.ErrorIs(t, err, domain.ErrInvalidStatus)
}

func TestAdUsecase_Close(t *testing.T) {
	repo := &adRepoStub{ownerID: 7, status: domain.AdStatusActive}

	listing, err := NewAdUsecase(repo, txFake{}).Close(context.Background(), 7, 10, domain.CloseReasonSoldElsewhere)
	require.NoError(t, err)
	require.Equal(t, domain.CloseReasonSoldElsewhere, repo.closed)
	require.Equal(t, int32(7), repo.viewerID)
	require.Equal(t, domain.AdStatusClosed, listing.Status)
}

func TestAdUsecase_Close_Errors(t *testing.T) {
	tests := []struct {
		name   string
		repo   *adRepoStub
		reason string
		want   error
	}{
		{"unknown reason", &adRepoStub{ownerID: 7, status: domain.AdStatusActive}, "bored", domain.ErrInvalidCloseReason},
		{"not owner", &adRepoStub{ownerID: 8, status: domain.AdStatusActive}, domain.CloseReasonOther, domain.ErrNotListingOwner},
		{"not active", &adRepoStub{ownerID: 7, status: domain.AdStatusDraft}, domain.CloseReasonOther, domain.ErrListingNotActive},
		{"missing", &adRepoStub{err: domain.ErrListingNotFound}, domain.CloseReasonOther, domain.ErrListingNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAdUsecase(tt.repo, txFake{}).Close(context.Background(), 7, 10, tt.reason)
			require.ErrorIs(t, err, tt.want)
			require.Empty(t, tt.repo.closed)
		})
	}
}

func TestAdUsecase_Delete(t *testing.T) {
	repo := &adRepoStub{ownerID: 7}
	require.NoError(t, NewAdUsecase(repo, txFake{}).Delete(context.Background(), 7, 10))
	require.True(t, repo.deleted)

	other := &adRepoStub{ownerID: 8}
	require.ErrorIs(t, NewAdUsecase(other, txFake{}).Delete(context.Background(), 7, 10), domain.ErrNotListingOwner)
	require.False(t, other.deleted)
}
