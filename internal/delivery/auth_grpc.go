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
}

func NewAuthGRPCServer(checker SessionChecker) *AuthGRPCServer {
	return &AuthGRPCServer{checker: checker}
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

var _ authv1.AuthServiceServer = (*AuthGRPCServer)(nil)
