package auth

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/auth/v1"
)

type fakeAuth struct {
	authv1.UnimplementedAuthServiceServer
	mu      sync.Mutex
	batches [][]int32
	session func(*authv1.CheckSessionRequest) (*authv1.CheckSessionResponse, error)
	users   func([]int32) (*authv1.GetUsersResponse, error)
}

func (f *fakeAuth) CheckSession(_ context.Context, r *authv1.CheckSessionRequest) (*authv1.CheckSessionResponse, error) {
	return f.session(r)
}

func (f *fakeAuth) GetUsers(_ context.Context, r *authv1.GetUsersRequest) (*authv1.GetUsersResponse, error) {
	f.mu.Lock()
	f.batches = append(f.batches, r.GetUserIds())
	f.mu.Unlock()
	return f.users(r.GetUserIds())
}

// echoUsers возвращает по карточке на каждый запрошенный ID.
func echoUsers(ids []int32) (*authv1.GetUsersResponse, error) {
	resp := &authv1.GetUsersResponse{}
	for _, id := range ids {
		resp.Users = append(resp.Users, &authv1.UserCard{UserId: id, FirstName: "u"})
	}
	return resp, nil
}

func newTestClient(t *testing.T, f *fakeAuth, timeout time.Duration) *Client {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	authv1.RegisterAuthServiceServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	c, err := New(lis.Addr().String(), timeout)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClient_Users_Batches(t *testing.T) {
	f := &fakeAuth{users: echoUsers}
	c := newTestClient(t, f, time.Second)
	ids := make([]int32, 250)
	for i := range ids {
		ids[i] = int32(i + 1)
	}

	got, err := c.Users(context.Background(), ids)

	require.NoError(t, err)
	require.Len(t, got, 250)
	require.Len(t, f.batches, 3)
	require.Len(t, f.batches[0], 100)
	require.Len(t, f.batches[1], 100)
	require.Len(t, f.batches[2], 50)
}

func TestClient_Users_EmptyAndInvalidIDsMakeNoCall(t *testing.T) {
	f := &fakeAuth{users: echoUsers}
	c := newTestClient(t, f, time.Second)

	for _, ids := range [][]int32{nil, {0, -1}} {
		got, err := c.Users(context.Background(), ids)
		require.NoError(t, err)
		require.Empty(t, got)
	}
	require.Empty(t, f.batches)
}

func TestClient_Users_DedupSorted(t *testing.T) {
	f := &fakeAuth{users: echoUsers}
	c := newTestClient(t, f, time.Second)

	got, err := c.Users(context.Background(), []int32{2, 1, 2, 1})

	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, [][]int32{{1, 2}}, f.batches)
}

func TestClient_Users_MapsCodes(t *testing.T) {
	cases := map[string]struct {
		users func([]int32) (*authv1.GetUsersResponse, error)
		want  error
	}{
		"unavailable": {func([]int32) (*authv1.GetUsersResponse, error) { return nil, status.Error(codes.Unavailable, "x") }, ErrUnavailable},
		"deadline":    {func([]int32) (*authv1.GetUsersResponse, error) { return nil, status.Error(codes.DeadlineExceeded, "x") }, ErrTimeout},
		"internal":    {func([]int32) (*authv1.GetUsersResponse, error) { return nil, status.Error(codes.Internal, "x") }, ErrInternal},
		"unrequested card": {func([]int32) (*authv1.GetUsersResponse, error) {
			return &authv1.GetUsersResponse{Users: []*authv1.UserCard{{UserId: 99}}}, nil
		}, ErrBadResponse},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := newTestClient(t, &fakeAuth{users: tc.users}, time.Second)
			_, err := c.Users(context.Background(), []int32{1})
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestClient_Users_FillsCard(t *testing.T) {
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	f := &fakeAuth{users: func([]int32) (*authv1.GetUsersResponse, error) {
		return &authv1.GetUsersResponse{Users: []*authv1.UserCard{{
			UserId: 4, FirstName: "Анна", Avatar: "4.png", CreatedUnixMicros: created.UnixMicro(),
			City: "Казань", PhoneVerified: true, Phone: "+79001234567",
		}}}, nil
	}}
	c := newTestClient(t, f, time.Second)

	got, err := c.Users(context.Background(), []int32{4})

	require.NoError(t, err)
	card := got[4]
	require.Equal(t, "Анна", card.FirstName)
	require.Equal(t, "Казань", card.City)
	require.True(t, card.PhoneVerified)
	require.Equal(t, "+79001234567", card.Phone)
	require.True(t, created.Equal(card.CreatedAt))
}

func TestClient_Me_Moderator(t *testing.T) {
	f := &fakeAuth{session: func(*authv1.CheckSessionRequest) (*authv1.CheckSessionResponse, error) {
		return &authv1.CheckSessionResponse{UserId: 7, EmailVerified: true, IsModerator: true}, nil
	}}
	c := newTestClient(t, f, time.Second)

	user, err := c.Me(context.Background(), "sid")

	require.NoError(t, err)
	require.Equal(t, int32(7), user.ID)
	require.True(t, user.IsModerator)
}
