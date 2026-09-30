package repository

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type AdRepository struct {
	mu  sync.RWMutex
	ads []domain.Ad
}

func NewAdRepository(ads []domain.Ad) *AdRepository {
	return &AdRepository{ads: ads}
}

func (r *AdRepository) List(_ context.Context, f domain.AdFilter) ([]domain.Ad, int, error) {
	r.mu.RLock()
	found := make([]domain.Ad, 0, len(r.ads))
	for _, ad := range r.ads {
		if matchesFilter(ad, f) {
			found = append(found, ad)
		}
	}
	r.mu.RUnlock()

	sortAds(found, f.Sort)

	total := len(found)
	if f.Offset >= total {
		return []domain.Ad{}, total, nil
	}
	end := min(f.Offset+f.Limit, total)

	return found[f.Offset:end], total, nil
}

func matchesFilter(ad domain.Ad, f domain.AdFilter) bool {
	if ad.Status != domain.AdStatusActive {
		return false
	}
	if f.CategoryID != 0 && ad.CategoryID != f.CategoryID {
		return false
	}
	if f.City != "" && !strings.EqualFold(ad.City, f.City) {
		return false
	}
	if f.PriceMin != nil && ad.Price < *f.PriceMin {
		return false
	}
	if f.PriceMax != nil && ad.Price > *f.PriceMax {
		return false
	}
	if f.HasDelivery != nil && ad.HasDelivery != *f.HasDelivery {
		return false
	}
	if f.Query != "" {
		q := strings.ToLower(f.Query)
		if !strings.Contains(strings.ToLower(ad.Title), q) && !strings.Contains(strings.ToLower(ad.Description), q) {
			return false
		}
	}
	return true
}

func sortAds(ads []domain.Ad, by string) {
	switch by {
	case domain.AdSortPriceAsc:
		slices.SortStableFunc(ads, func(a, b domain.Ad) int { return cmp.Compare(a.Price, b.Price) })
	case domain.AdSortPriceDesc:
		slices.SortStableFunc(ads, func(a, b domain.Ad) int { return cmp.Compare(b.Price, a.Price) })
	default:
		slices.SortStableFunc(ads, func(a, b domain.Ad) int { return b.CreatedAt.Compare(a.CreatedAt) })
	}
}
