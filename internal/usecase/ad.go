package usecase

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
)

var (
	ErrInvalidPriceFormat = errors.New("invalid price format")
	ErrInvalidPriceRange  = errors.New("priceMax must not be less than priceMin")
	ErrInvalidSort        = errors.New("invalid sort value")
	ErrInvalidAdType      = errors.New("invalid ad type")
)

const (
	defaultAdsLimit = 24
	maxAdsLimit     = 60
	maxAdsOffset    = 10000
)

var (
	priceFormat  = regexp.MustCompile(`^\d{1,10}(\.\d{1,2})?$`)
	allowedSorts = map[string]bool{"": true, "newest": true, "price_asc": true, "price_desc": true}
	allowedTypes = map[string]bool{"": true, "sell": true, "buy": true, "service": true}
)

type ListAdsParams struct {
	Query       string
	CategoryID  *int32
	City        string
	PriceMin    string
	PriceMax    string
	HasDelivery *bool
	Type        string
	Sort        string
	Limit       int
	Offset      int
}

type AdUsecase struct {
	ads *repository.AdRepository
}

func NewAdUsecase(ads *repository.AdRepository) *AdUsecase {
	return &AdUsecase{ads: ads}
}

func (uc *AdUsecase) List(ctx context.Context, p ListAdsParams, viewerID *int32) (domain.AdPage, error) {
	p.Query = strings.TrimSpace(p.Query)
	p.City = normalizeCity(p.City)

	if !allowedSorts[p.Sort] {
		return domain.AdPage{}, ErrInvalidSort
	}
	if !allowedTypes[p.Type] {
		return domain.AdPage{}, ErrInvalidAdType
	}
	if p.PriceMin != "" && !priceFormat.MatchString(p.PriceMin) {
		return domain.AdPage{}, ErrInvalidPriceFormat
	}
	if p.PriceMax != "" && !priceFormat.MatchString(p.PriceMax) {
		return domain.AdPage{}, ErrInvalidPriceFormat
	}
	if p.PriceMin != "" && p.PriceMax != "" {
		min, _ := strconv.ParseFloat(p.PriceMin, 64)
		max, _ := strconv.ParseFloat(p.PriceMax, 64)
		if max < min {
			return domain.AdPage{}, ErrInvalidPriceRange
		}
	}

	limit := p.Limit
	if limit <= 0 {
		limit = defaultAdsLimit
	}
	if limit > maxAdsLimit {
		limit = maxAdsLimit
	}

	offset := p.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > maxAdsOffset {
		offset = maxAdsOffset
	}

	items, total, err := uc.ads.List(ctx, repository.AdListFilter{
		Query:       p.Query,
		CategoryID:  p.CategoryID,
		City:        p.City,
		PriceMin:    p.PriceMin,
		PriceMax:    p.PriceMax,
		HasDelivery: p.HasDelivery,
		Type:        p.Type,
		Sort:        p.Sort,
		Limit:       limit,
		Offset:      offset,
		ViewerID:    viewerID,
	})
	if err != nil {
		return domain.AdPage{}, err
	}

	return domain.AdPage{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func normalizeCity(city string) string {
	return strings.ToLower(strings.Join(strings.Fields(city), " "))
}
