package delivery

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type AdHandler struct {
	ads AdUsecase
}

func NewAdHandler(ads AdUsecase) *AdHandler {
	return &AdHandler{ads: ads}
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func adResponse(ad domain.Ad) map[string]any {
	var price any
	if ad.Price != nil {
		price = formatPrice(*ad.Price)
	}

	return map[string]any{
		"id":           ad.ID,
		"title":        ad.Title,
		"price":        price,
		"is_free":      ad.IsFree,
		"image_url":    nullable(ad.ImageURL),
		"city":         ad.City,
		"has_delivery": ad.HasDelivery,
		"category_id":  ad.CategoryID,
		"created_at":   ad.CreatedAt,
	}
}

func (h *AdHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	params := domain.ListAdsParams{
		Query:    q.Get("q"),
		City:     q.Get("city"),
		PriceMin: q.Get("price_min"),
		PriceMax: q.Get("price_max"),
		Sort:     q.Get("sort"),
	}

	categoryID, err := intParam(q, "category_id", 1)
	if err != nil || categoryID > math.MaxInt32 {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid category_id")
		return
	}
	params.CategoryID = int32(categoryID)

	if params.Limit, err = intParam(q, "limit", 1); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	if params.Offset, err = intParam(q, "offset", 0); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	if v := q.Get("has_delivery"); v != "" {
		delivery, err := strconv.ParseBool(v)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid has_delivery")
			return
		}
		params.HasDelivery = &delivery
	}

	page, err := h.ads.List(r.Context(), params)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidPrice),
			errors.Is(err, domain.ErrInvalidPriceRange),
			errors.Is(err, domain.ErrInvalidSort):
			writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		default:
			slog.ErrorContext(r.Context(), "list ads", "err", err)
			writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, adPageResponse(page, adResponse))
}

func adPageResponse(page domain.AdPage, card func(domain.Ad) map[string]any) map[string]any {
	items := make([]map[string]any, 0, len(page.Items))
	for _, ad := range page.Items {
		items = append(items, card(ad))
	}
	return map[string]any{
		"items":  items,
		"total":  page.Total,
		"limit":  page.Limit,
		"offset": page.Offset,
	}
}

func myAdResponse(ad domain.Ad) map[string]any {
	resp := adResponse(ad)
	resp["status"] = ad.Status
	resp["close_reason"] = nullable(ad.CloseReason)
	resp["updated_at"] = ad.UpdatedAt
	return resp
}

func (h *AdHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := domain.MyAdFilter{UserID: userIDFromContext(r.Context()), Status: q.Get("status")}

	var err error
	if f.Limit, err = intParam(q, "limit", 1); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	if f.Offset, err = intParam(q, "offset", 0); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.ads.ListMine(r.Context(), f)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidStatus) {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}
		slog.ErrorContext(r.Context(), "list my ads", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := adPageResponse(page.AdPage, myAdResponse)
	resp["counts"] = page.Counts
	writeJSON(r.Context(), w, http.StatusOK, resp)
}

func intParam(q url.Values, name string, minValue int) (int, error) {
	v := q.Get(name)
	if v == "" {
		return 0, nil
	}

	n, err := strconv.Atoi(v)
	if err != nil || n < minValue {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return n, nil
}
