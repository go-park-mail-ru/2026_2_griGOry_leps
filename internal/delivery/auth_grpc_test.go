package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	authv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/auth/v1"
)

func newAuthGRPC(t *testing.T) (*AuthGRPCServer, *MockSessionChecker, *MockUserLookup) {
	t.Helper()
	ctrl := gomock.NewController(t)
	checker, users := NewMockSessionChecker(ctrl), NewMockUserLookup(ctrl)
	return NewAuthGRPCServer(checker, users), checker, users
}

func TestAuthGRPC_CheckSession_Moderator(t *testing.T) {
	srv, checker, _ := newAuthGRPC(t)
	checker.EXPECT().Me(gomock.Any(), "sid").Return(domain.User{ID: 7, EmailVerified: true, IsModerator: true}, nil)

	resp, err := srv.CheckSession(context.Background(), &authv1.CheckSessionRequest{SessionId: "sid"})

	require.NoError(t, err)
	require.Equal(t, int32(7), resp.UserId)
	require.True(t, resp.EmailVerified)
	require.True(t, resp.IsModerator)
}

func TestAuthGRPC_GetUsers_InvalidArguments(t *testing.T) {
	srv, _, _ := newAuthGRPC(t)
	tooMany := make([]int32, 101)
	for i := range tooMany {
		tooMany[i] = int32(i + 1)
	}
	for name, ids := range map[string][]int32{"more than 100": tooMany, "zero id": {0}, "negative id": {-4}} {
		t.Run(name, func(t *testing.T) {
			_, err := srv.GetUsers(context.Background(), &authv1.GetUsersRequest{UserIds: ids})
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestAuthGRPC_GetUsers_DedupAndMapping(t *testing.T) {
	srv, _, users := newAuthGRPC(t)
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	users.EXPECT().Users(gomock.Any(), []int32{3, 5}).Return([]domain.UserCard{
		{ID: 3, FirstName: "Анна", Avatar: "3.png", CreatedAt: created, City: "Казань", PhoneVerified: true, Phone: "+79001234567"},
	}, nil)

	resp, err := srv.GetUsers(context.Background(), &authv1.GetUsersRequest{UserIds: []int32{3, 3, 5}})

	require.NoError(t, err)
	require.Len(t, resp.Users, 1)
	u := resp.Users[0]
	require.Equal(t, int32(3), u.UserId)
	require.Equal(t, "Анна", u.FirstName)
	require.Equal(t, "Казань", u.City)
	require.Equal(t, "+79001234567", u.Phone)
	require.True(t, u.PhoneVerified)
	require.Equal(t, created.UnixMicro(), u.CreatedUnixMicros)
}
