package delivery

import (
	"context"
	"errors"
	"log/slog"
	"math"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	adsv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/ads/v1"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
)

type AdsGRPCServer struct {
	adsv1.UnimplementedAdsServiceServer
	ads AdUsecase
}

func NewAdsGRPCServer(ads AdUsecase) *AdsGRPCServer {
	return &AdsGRPCServer{ads: ads}
}

func (s *AdsGRPCServer) SearchPublished(
	ctx context.Context,
	req *adsv1.SearchRequest,
) (*adsv1.SearchPage, error) {
	if ids := metadata.ValueFromIncomingContext(ctx, "x-request-id"); len(ids) == 1 {
		ctx = logger.WithRequestID(ctx, logger.NormalizeRequestID(ids[0]))
	}

	if req.Limit < 0 || req.Offset < 0 ||
		req.Limit > math.MaxInt || req.Offset > math.MaxInt {
		return nil, status.Error(codes.InvalidArgument, "invalid pagination")
	}

	page, err := s.ads.List(ctx, domain.ListAdsParams{
		Query:  req.Query,
		Sort:   domain.AdSortNewest,
		Limit:  int(req.Limit),
		Offset: int(req.Offset),
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, status.FromContextError(err).Err()
		}

		slog.ErrorContext(ctx, "search published ads", "err", err)
		return nil, status.Error(codes.Internal, "search failed")
	}

	result := &adsv1.SearchPage{
		Total:  int64(page.Total),
		Limit:  int64(page.Limit),
		Offset: int64(page.Offset),
	}

	for _, ad := range page.Items {
		result.Items = append(result.Items, &adsv1.SearchCard{
			Id:                  ad.ID,
			Title:               ad.Title,
			PriceKopecks:        ad.Price,
			IsFree:              ad.IsFree,
			ImageUrl:            ad.ImageURL,
			City:                ad.City,
			HasDelivery:         ad.HasDelivery,
			CategoryId:          ad.CategoryID,
			PublishedUnixMicros: ad.CreatedAt.UnixMicro(),
		})
	}

	return result, nil
}
