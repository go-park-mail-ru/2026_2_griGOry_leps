package handler

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
)

type AdHandler struct {
	ads *usecase.AdUsecase
}

func NewAdHandler(ads *usecase.AdUsecase) *AdHandler {
	return &AdHandler{ads: ads}
}

func adResponse(ad domain.Ad) map[string]any {
	var imageURL any
	if ad.ImageURL != "" {
		imageURL = ad.ImageURL
	}

	return map[string]any{
		"id":           ad.ID,
		"title":        ad.Title,
		"price":        fmt.Sprintf("%d.%02d", ad.Price/100, ad.Price%100),
		"image_url":    imageURL,
		"city":         ad.City,
		"has_delivery": ad.HasDelivery,
		"category_id":  ad.CategoryID,
		"created_at":   ad.CreatedAt,
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

	categoryID, err := intParam(q, "category_id")
	if err != nil || categoryID > math.MaxInt32 {
		writeError(w, http.StatusBadRequest, "invalid category_id")
		return
	}
	params.CategoryID = int32(categoryID)

	if params.Limit, err = intParam(q, "limit"); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if params.Offset, err = intParam(q, "offset"); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if v := q.Get("has_delivery"); v != "" {
		delivery, err := strconv.ParseBool(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid has_delivery")
			return
		}
		params.HasDelivery = &delivery
	}

	page, err := h.ads.List(r.Context(), params)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidPrice),
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
		items = append(items, adResponse(ad))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  page.Total,
		"limit":  page.Limit,
		"offset": page.Offset,
	})
}

// intParam читает неотрицательное целое из query; отсутствующий параметр даёт 0.
func intParam(q url.Values, name string) (int, error) {
	v := q.Get(name)
	if v == "" {
		return 0, nil
	}

	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return n, nil
}
