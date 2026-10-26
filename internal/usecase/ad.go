package usecase

import (
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	defaultAdsLimit = 24
	maxAdsLimit     = 60
)

var priceFormat = regexp.MustCompile(`^(\d{1,10})(?:\.(\d{1,2}))?$`)

type AdUsecase struct {
	ads AdRepository
	tx  Transactor
}

func NewAdUsecase(ads AdRepository, tx Transactor) *AdUsecase {
	return &AdUsecase{ads: ads, tx: tx}
}

func pageLimit(limit int) int {
	if limit <= 0 {
		return defaultAdsLimit
	}
	return min(limit, maxAdsLimit)
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
		return domain.AdPage{}, domain.ErrInvalidSort
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
		return domain.AdPage{}, domain.ErrInvalidPriceRange
	}

	f.Limit = pageLimit(f.Limit)

	items, total, err := uc.ads.List(ctx, f)
	if err != nil {
		return domain.AdPage{}, err
	}

	return domain.AdPage{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (uc *AdUsecase) ListMine(ctx context.Context, f domain.MyAdFilter) (domain.MyAdPage, error) {
	if f.Status != "" && !slices.Contains(domain.AdStatuses, f.Status) {
		return domain.MyAdPage{}, domain.ErrInvalidStatus
	}
	f.Limit = pageLimit(f.Limit)

	items, byStatus, err := uc.ads.ListByUser(ctx, f)
	if err != nil {
		return domain.MyAdPage{}, err
	}

	counts := map[string]int{"all": 0}
	for _, status := range domain.AdStatuses {
		counts[status] = byStatus[status]
		counts["all"] += byStatus[status]
	}

	total := counts["all"]
	if f.Status != "" {
		total = counts[f.Status]
	}
	return domain.MyAdPage{
		AdPage: domain.AdPage{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset},
		Counts: counts,
	}, nil
}

var closeReasons = []string{
	domain.CloseReasonSoldOnGoget, domain.CloseReasonSoldElsewhere, domain.CloseReasonChangedMind, domain.CloseReasonOther,
}

func (uc *AdUsecase) Close(ctx context.Context, userID, id int32, reason string) (domain.Listing, error) {
	if !slices.Contains(closeReasons, reason) {
		return domain.Listing{}, domain.ErrInvalidCloseReason
	}

	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		ownerID, status, err := uc.ads.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if ownerID != userID {
			return domain.ErrNotListingOwner
		}
		if status != domain.AdStatusActive {
			return domain.ErrListingNotActive
		}
		return uc.ads.Close(ctx, id, reason)
	})
	if err != nil {
		return domain.Listing{}, err
	}
	return uc.ads.GetListing(ctx, id, userID)
}

func (uc *AdUsecase) Delete(ctx context.Context, userID, id int32) error {
	return uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		ownerID, _, err := uc.ads.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if ownerID != userID {
			return domain.ErrNotListingOwner
		}
		return uc.ads.Delete(ctx, id)
	})
}

// parsePrice переводит цену в рублях ("1500" или "1500.5") в копейки.
func parsePrice(s string) (int64, error) {
	m := priceFormat.FindStringSubmatch(s)
	if m == nil {
		return 0, domain.ErrInvalidPrice
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
