package delivery

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	authv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/auth/v1"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AuthGRPCServer struct {
	authv1.UnimplementedAuthServiceServer
	checker SessionChecker
	users   UserLookup
}

func NewAuthGRPCServer(checker SessionChecker, users UserLookup) *AuthGRPCServer {
	return &AuthGRPCServer{checker: checker, users: users}
}

func (s *AuthGRPCServer) CheckSession(ctx context.Context, req *authv1.CheckSessionRequest) (resp *authv1.CheckSessionResponse, err error) {
	var id string
	if values := metadata.ValueFromIncomingContext(ctx, "x-request-id"); len(values) == 1 {
		id = values[0]
	}
	ctx = logger.WithRequestID(ctx, logger.NormalizeRequestID(id))
	defer func() {
		slog.InfoContext(ctx, "CheckSession", "code", status.Code(err).String())
	}()
	sessionID := req.GetSessionId()
	if sessionID == "" {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	user, err := s.checker.Me(ctx, sessionID)
	switch {
	case err == nil:
		return &authv1.CheckSessionResponse{
			UserId:        user.ID,
			EmailVerified: user.EmailVerified,
			IsModerator:   user.IsModerator,
		}, nil
	case errors.Is(err, domain.ErrSessionExpired), errors.Is(err, domain.ErrUserNotFound):
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return nil, status.FromContextError(err).Err()
	default:
		slog.ErrorContext(ctx, "check session", "err", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
}

const maxGetUsers = 100

func (s *AuthGRPCServer) GetUsers(ctx context.Context, req *authv1.GetUsersRequest) (resp *authv1.GetUsersResponse, err error) {
	if values := metadata.ValueFromIncomingContext(ctx, "x-request-id"); len(values) == 1 {
		ctx = logger.WithRequestID(ctx, logger.NormalizeRequestID(values[0]))
	}
	defer func() {
		slog.InfoContext(ctx, "GetUsers", "code", status.Code(err).String())
	}()
	ids, ok := uniqueUserIDs(req.GetUserIds())
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid user ids")
	}
	cards, err := s.users.Users(ctx, ids)
	switch {
	case err == nil:
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return nil, status.FromContextError(err).Err()
	default:
		slog.ErrorContext(ctx, "get users", "err", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	resp = &authv1.GetUsersResponse{Users: make([]*authv1.UserCard, 0, len(cards))}
	for _, c := range cards {
		resp.Users = append(resp.Users, &authv1.UserCard{
			UserId:            c.ID,
			FirstName:         c.FirstName,
			Avatar:            c.Avatar,
			CreatedUnixMicros: c.CreatedAt.UnixMicro(),
			City:              c.City,
			PhoneVerified:     c.PhoneVerified,
			Phone:             c.Phone,
		})
	}
	return resp, nil
}

// uniqueUserIDs схлопывает повторы с сохранением порядка; false — ID некорректны или их больше maxGetUsers.
func uniqueUserIDs(ids []int32) ([]int32, bool) {
	seen := make(map[int32]struct{}, len(ids))
	out := make([]int32, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, false
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, len(out) <= maxGetUsers
}

var _ authv1.AuthServiceServer = (*AuthGRPCServer)(nil)
