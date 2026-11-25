package usecase

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
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
		return domain.AdPage{}, domain.ErrInvalidSort
	}
	if p.Cursor != "" {
		if f.Sort != domain.AdSortNewest || f.Offset != 0 {
			return domain.AdPage{}, domain.ErrInvalidCursor
		}
		cursorTime, cursorID, err := parseAdCursor(p.Cursor)
		if err != nil {
			return domain.AdPage{}, err
		}
		f.CursorTime, f.CursorID = &cursorTime, cursorID
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

	page := domain.AdPage{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset}
	if f.Sort == domain.AdSortNewest && len(items) == f.Limit {
		last := items[len(items)-1]
		page.NextCursor = encodeAdCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

func encodeAdCursor(at time.Time, id int32) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%d", at.UnixMicro(), id)))
}

func parseAdCursor(cursor string) (time.Time, int32, error) {
	if len(cursor) > 64 {
		return time.Time{}, 0, domain.ErrInvalidCursor
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, 0, domain.ErrInvalidCursor
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 {
		return time.Time{}, 0, domain.ErrInvalidCursor
	}
	micros, timeErr := strconv.ParseInt(parts[0], 10, 64)
	id, idErr := strconv.ParseInt(parts[1], 10, 32)
	if timeErr != nil || idErr != nil || micros <= 0 || id <= 0 {
		return time.Time{}, 0, domain.ErrInvalidCursor
	}
	return time.UnixMicro(micros).UTC(), int32(id), nil
}

func pageLimit(limit int) int {
	if limit <= 0 {
		return defaultAdsLimit
	}
	return min(limit, maxAdsLimit)
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
