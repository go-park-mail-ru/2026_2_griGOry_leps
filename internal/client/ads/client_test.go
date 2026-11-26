package ads

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	adsv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/ads/v1"
)

type fakeAds struct {
	adsv1.UnimplementedAdsServiceServer
	stats func(*adsv1.SellerStatsRequest) (*adsv1.SellerStats, error)
}

func (f *fakeAds) GetSellerStats(_ context.Context, r *adsv1.SellerStatsRequest) (*adsv1.SellerStats, error) {
	return f.stats(r)
}

func newTestClient(t *testing.T, f *fakeAds) *Client {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	adsv1.RegisterAdsServiceServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	c, err := New(lis.Addr().String(), time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClient_SellerStats(t *testing.T) {
	c := newTestClient(t, &fakeAds{stats: func(r *adsv1.SellerStatsRequest) (*adsv1.SellerStats, error) {
		require.Equal(t, int32(5), r.UserId)
		return &adsv1.SellerStats{Rating: 4.5, ReviewsCount: 2}, nil
	}})

	got, err := c.SellerStats(context.Background(), 5)

	require.NoError(t, err)
	require.InDelta(t, 4.5, got.Rating, 0.001)
	require.Equal(t, 2, got.ReviewsCount)
}

func TestClient_SellerStats_MapsErrors(t *testing.T) {
	cases := map[string]struct {
		resp *adsv1.SellerStats
		err  error
		want error
	}{
		"unavailable":    {err: status.Error(codes.Unavailable, "x"), want: ErrUnavailable},
		"deadline":       {err: status.Error(codes.DeadlineExceeded, "x"), want: ErrTimeout},
		"internal":       {err: status.Error(codes.Internal, "x"), want: ErrInternal},
		"rating range":   {resp: &adsv1.SellerStats{Rating: 7}, want: ErrBadResponse},
		"negative count": {resp: &adsv1.SellerStats{ReviewsCount: -1}, want: ErrBadResponse},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := newTestClient(t, &fakeAds{stats: func(*adsv1.SellerStatsRequest) (*adsv1.SellerStats, error) { return tc.resp, tc.err }})
			_, err := c.SellerStats(context.Background(), 5)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestClient_SellerStats_InvalidUser(t *testing.T) {
	c := newTestClient(t, &fakeAds{})
	_, err := c.SellerStats(context.Background(), 0)
	require.ErrorIs(t, err, ErrBadResponse)
}
