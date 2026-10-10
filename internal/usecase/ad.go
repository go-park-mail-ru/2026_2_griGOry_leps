package usecase

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

var (
	ErrInvalidPrice      = errors.New("invalid price")
	ErrInvalidPriceRange = errors.New("price_max must not be less than price_min")
	ErrInvalidSort       = errors.New("invalid sort value")
)

const (
	defaultAdsLimit = 24
	maxAdsLimit     = 60
)

var priceFormat = regexp.MustCompile(`^(\d{1,10})(?:\.(\d{1,2}))?$`)

type AdUsecase struct {
	ads AdRepository
}

func NewAdUsecase(ads AdRepository) *AdUsecase {
	return &AdUsecase{ads: ads}
}

func (uc *AdUsecase) List(ctx context.Context, p domain.ListAdsParams) (domain.AdPage, error) {
	f := domain.AdFilter{
		Query:       strings.TrimSpace(p.Query),
		CategoryID:  p.CategoryID,
		City:        strings.TrimSpace(p.City),
		HasDelivery: p.HasDelivery,
		Sort:        p.Sort,
		Limit:       p.Limit,
		Offset:      p.Offset,
	}

	switch f.Sort {
	case "":
		f.Sort = domain.AdSortNewest
	case domain.AdSortNewest, domain.AdSortPriceAsc, domain.AdSortPriceDesc:
	default:
		return domain.AdPage{}, ErrInvalidSort
	}

	if p.PriceMin != "" {
		v, err := parsePrice(p.PriceMin)
		if err != nil {
			return domain.AdPage{}, err
		}
		f.PriceMin = &v
	}
	if p.PriceMax != "" {
		v, err := parsePrice(p.PriceMax)
		if err != nil {
			return domain.AdPage{}, err
		}
		f.PriceMax = &v
	}
	if f.PriceMin != nil && f.PriceMax != nil && *f.PriceMax < *f.PriceMin {
		return domain.AdPage{}, ErrInvalidPriceRange
	}

	if f.Limit <= 0 {
		f.Limit = defaultAdsLimit
	}
	f.Limit = min(f.Limit, maxAdsLimit)

	items, total, err := uc.ads.List(ctx, f)
	if err != nil {
		return domain.AdPage{}, err
	}

	return domain.AdPage{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

// parsePrice переводит цену в рублях ("1500" или "1500.5") в копейки.
func parsePrice(s string) (int64, error) {
	m := priceFormat.FindStringSubmatch(s)
	if m == nil {
		return 0, ErrInvalidPrice
	}

	rubles, _ := strconv.ParseInt(m[1], 10, 64)

	var kopecks int64
	if m[2] != "" {
		frac := m[2]
		if len(frac) == 1 {
			frac += "0"
		}
		kopecks, _ = strconv.ParseInt(frac, 10, 64)
	}

	return rubles*100 + kopecks, nil
}
