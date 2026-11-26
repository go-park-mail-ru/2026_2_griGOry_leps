package delivery

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	adsv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/ads/v1"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
)

func NewSearchRouter(frontendOrigin string, ads adsv1.AdsServiceClient) http.Handler {
	r := newAPIRouter()

	r.HandleFunc("/api/health", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(req.Context(), w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "search",
		})
	}).Methods(http.MethodGet)

	r.HandleFunc("/api/search", searchPublished(ads)).Methods(http.MethodGet)

	timed := http.TimeoutHandler(r, requestTimeout, `{"error":"request timeout"}`)
	return requestIDMiddleware(accessLogMiddleware(corsMiddleware(frontendOrigin, jsonContentType(timed))))
}

func searchPublished(ads adsv1.AdsServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params, err := parseAdListParams(r.URL.RawQuery)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", logger.RequestID(ctx))
		result, err := ads.SearchPublished(ctx, &adsv1.SearchRequest{
			Query:       params.Query,
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
}
