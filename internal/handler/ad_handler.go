package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
)

type AdHandler struct {
	ads  *usecase.AdUsecase
	auth *usecase.AuthUsecase
}

func NewAdHandler(ads *usecase.AdUsecase, auth *usecase.AuthUsecase) *AdHandler {
	return &AdHandler{ads: ads, auth: auth}
}

func adCardResponse(ad domain.AdCard) map[string]any {
	return map[string]any{
		"id":            ad.ID,
		"title":         ad.Title,
		"price":         ad.Price,
		"image_url":     ad.ImageURL,
		"city":          ad.City,
		"has_delivery":  ad.HasDelivery,
		"category_id":   ad.CategoryID,
		"rating":        ad.Rating,
		"reviews_count": ad.ReviewsCount,
		"is_favorite":   ad.IsFavorite,
		"created_at":    ad.CreatedAt,
	}
}

func (h *AdHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	params := usecase.ListAdsParams{
		Query:    q.Get("q"),
		City:     q.Get("city"),
		PriceMin: q.Get("price_min"),
		PriceMax: q.Get("price_max"),
		Sort:     q.Get("sort"),
	}

	if v := q.Get("category_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 32)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid category_id")
			return
		}
		id32 := int32(id)
		params.CategoryID = &id32
	}

	if v := q.Get("has_delivery"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid has_delivery")
			return
		}
		params.HasDelivery = &b
	}

	if v := q.Get("limit"); v != "" {
		limit, err := strconv.Atoi(v)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		params.Limit = limit
	}

	if v := q.Get("offset"); v != "" {
		offset, err := strconv.Atoi(v)
		if err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		params.Offset = offset
	}

	var viewerID *int32
	if cookie, err := r.Cookie("session_id"); err == nil {
		if user, err := h.auth.Me(r.Context(), cookie.Value); err == nil {
			viewerID = &user.ID
		}
	}

	page, err := h.ads.List(r.Context(), params, viewerID)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidPriceFormat),
			errors.Is(err, usecase.ErrInvalidPriceRange),
			errors.Is(err, usecase.ErrInvalidSort):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("list ads error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	items := make([]map[string]any, 0, len(page.Items))
	for _, ad := range page.Items {
		items = append(items, adCardResponse(ad))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  page.Total,
		"limit":  page.Limit,
		"offset": page.Offset,
	})
}
