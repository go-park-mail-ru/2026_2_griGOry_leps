package delivery

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	adsv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/ads/v1"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
)

func NewSearchRouter(frontendOrigin string, ads adsv1.AdsServiceClient) http.Handler {
	r := mux.NewRouter()
	r.Use(recoverMiddleware)
	r.NotFoundHandler = http.HandlerFunc(notFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(methodNotAllowed)

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
		q := r.URL.Query()

		limit, err := intParam(q, "limit", 1)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}

		offset, err := intParam(q, "offset", 0)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", logger.RequestID(ctx))
		result, err := ads.SearchPublished(ctx, &adsv1.SearchRequest{
			Query:  q.Get("q"),
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			switch status.Code(err) {
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
			Total:  int(result.Total),
			Limit:  int(result.Limit),
			Offset: int(result.Offset),
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

		writeJSON(ctx, w, http.StatusOK, adPageResponse(page, adResponse))
	}
}
