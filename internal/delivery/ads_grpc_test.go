package delivery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	adsv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/ads/v1"
)

func TestAdsGRPC_GetSellerStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	sellers := NewMockSellerUsecase(ctrl)
	srv := NewAdsGRPCServer(NewMockAdUsecase(ctrl), sellers)
	sellers.EXPECT().Stats(gomock.Any(), int32(5)).Return(domain.SellerStats{Rating: 4.5, ReviewsCount: 2}, nil)

	resp, err := srv.GetSellerStats(context.Background(), &adsv1.SellerStatsRequest{UserId: 5})

	require.NoError(t, err)
	require.InDelta(t, 4.5, resp.Rating, 0.001)
	require.Equal(t, int64(2), resp.ReviewsCount)
}

func TestAdsGRPC_GetSellerStats_InvalidUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	srv := NewAdsGRPCServer(NewMockAdUsecase(ctrl), NewMockSellerUsecase(ctrl))

	_, err := srv.GetSellerStats(context.Background(), &adsv1.SellerStatsRequest{UserId: 0})

	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
