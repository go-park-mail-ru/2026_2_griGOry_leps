package ads

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	adsv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/ads/v1"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
)

var (
	ErrUnavailable = errors.New("ads service unavailable")
	ErrTimeout     = errors.New("ads service timeout")
	ErrBadResponse = errors.New("invalid ads service response")
	ErrInternal    = errors.New("ads service internal error")
)

type Client struct {
	conn    *grpc.ClientConn
	ads     adsv1.AdsServiceClient
	timeout time.Duration
}

func New(addr string, timeout time.Duration) (*Client, error) {
	if addr == "" {
		return nil, errors.New("ads gRPC address must be set")
	}
	if timeout <= 0 {
		return nil, errors.New("ads timeout must be positive")
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, ads: adsv1.NewAdsServiceClient(conn), timeout: timeout}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) SellerStats(ctx context.Context, userID int32) (domain.SellerStats, error) {
	if userID <= 0 {
		return domain.SellerStats{}, ErrBadResponse
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if id := logger.RequestID(ctx); id != "" {
		callCtx = metadata.AppendToOutgoingContext(callCtx, "x-request-id", id)
	}

	resp, err := c.ads.GetSellerStats(callCtx, &adsv1.SellerStatsRequest{UserId: userID})
	if err != nil {
		if ctx.Err() != nil {
			return domain.SellerStats{}, ctx.Err()
		}
		switch status.Code(err) {
		case codes.Unavailable:
			return domain.SellerStats{}, ErrUnavailable
		case codes.DeadlineExceeded:
			return domain.SellerStats{}, ErrTimeout
		case codes.Internal:
			return domain.SellerStats{}, ErrInternal
		default:
			return domain.SellerStats{}, ErrBadResponse
		}
	}
	if resp.GetRating() < 0 || resp.GetRating() > 5 || resp.GetReviewsCount() < 0 {
		return domain.SellerStats{}, ErrBadResponse
	}
	return domain.SellerStats{Rating: resp.GetRating(), ReviewsCount: int(resp.GetReviewsCount())}, nil
}
