package delivery

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	adsv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/ads/v1"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/onlineconf"
)

type searchHandler struct {
	ads      adsv1.AdsServiceClient
	sessions *SessionMiddleware
	settings *onlineconf.SearchCache
}

func NewSearchRouter(frontendOrigin string, ads adsv1.AdsServiceClient, sessions *SessionMiddleware, settings *onlineconf.SearchCache) http.Handler {
	r := newAPIRouter()

	r.HandleFunc("/api/health", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(req.Context(), w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "search",
		})
	}).Methods(http.MethodGet)

	search := searchHandler{ads: ads, sessions: sessions, settings: settings}
	r.HandleFunc("/api/search", search.published).Methods(http.MethodGet)
	r.HandleFunc("/api/search/settings", search.options).Methods(http.MethodGet)

	timed := http.TimeoutHandler(r, requestTimeout, `{"error":"request timeout"}`)
	return requestIDMiddleware(accessLogMiddleware(corsMiddleware(frontendOrigin, jsonContentType(timed))))
}

func (h searchHandler) options(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	snapshot := h.settings.Snapshot()
	enabled, err := h.priceSortEnabled(r, snapshot)
	if err != nil {
		writeSessionError(w, r, err)
		return
	}
	response := struct {
		MaxQueryLength    *int32 `json:"max_query_length"`
		PriceSortEnabled  bool   `json:"price_sort_enabled"`
		SearchButtonColor string `json:"search_button_color"`
	}{PriceSortEnabled: enabled, SearchButtonColor: snapshot.ButtonColor()}
	if snapshot.MaxQueryLength > 0 {
		response.MaxQueryLength = &snapshot.MaxQueryLength
	}
	writeJSON(r.Context(), w, http.StatusOK, response)
}

func (h searchHandler) priceSortEnabled(r *http.Request, snapshot onlineconf.SearchSnapshot) (bool, error) {
	if !snapshot.PriceSortDependsOnUser() {
		return snapshot.PriceSortEnabled(0), nil
	}
	userID, err := h.sessions.optionalUserID(r)
	return snapshot.PriceSortEnabled(userID), err
}

func (h searchHandler) published(w http.ResponseWriter, r *http.Request) {
	params, err := parseAdListParams(r.URL.RawQuery)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	snapshot := h.settings.Snapshot()
	query := strings.TrimSpace(params.Query)
	if snapshot.MaxQueryLength > 0 && utf8.RuneCountInString(query) > int(snapshot.MaxQueryLength) {
		writeFieldError(r.Context(), w, http.StatusBadRequest, "q", "search query too long")
		return
	}
	if params.Sort == domain.AdSortPriceAsc || params.Sort == domain.AdSortPriceDesc {
		w.Header().Set("Cache-Control", "no-store")
		enabled, err := h.priceSortEnabled(r, snapshot)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		if !enabled {
			writeError(r.Context(), w, http.StatusForbidden, "price sorting disabled")
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", logger.RequestID(ctx))
	result, err := h.ads.SearchPublished(ctx, &adsv1.SearchRequest{
		Query:       query,
		CategoryId:  params.CategoryID,
		City:        params.City,
		PriceMin:    params.PriceMin,
		PriceMax:    params.PriceMax,
		HasDelivery: params.HasDelivery,
		Sort:        params.Sort,
		Limit:       int64(params.Limit),
		Offset:      int64(params.Offset),
		Cursor:      params.Cursor,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.InvalidArgument:
			writeError(ctx, w, http.StatusBadRequest, status.Convert(err).Message())
		case codes.Unavailable:
			writeError(ctx, w, http.StatusServiceUnavailable, "ads unavailable")
		case codes.DeadlineExceeded:
			writeError(ctx, w, http.StatusGatewayTimeout, "ads timeout")
		default:
			writeError(ctx, w, http.StatusBadGateway, "ads request failed")
		}
		return
	}

	page := domain.AdPage{
		Total:      int(result.Total),
		Limit:      int(result.Limit),
		Offset:     int(result.Offset),
		NextCursor: result.NextCursor,
	}

	for _, card := range result.Items {
		page.Items = append(page.Items, domain.Ad{
			ID:          card.Id,
			Title:       card.Title,
			Price:       card.PriceKopecks,
			IsFree:      card.IsFree,
			ImageURL:    card.ImageUrl,
			City:        card.City,
			HasDelivery: card.HasDelivery,
			CategoryID:  card.CategoryId,
			CreatedAt:   time.UnixMicro(card.PublishedUnixMicros).UTC(),
		})
	}

	writeJSON(ctx, w, http.StatusOK, publicAdPageResponse(page))
}
