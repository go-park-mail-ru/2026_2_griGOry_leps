package auth

import (
	"context"
	"errors"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	authv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/auth/v1"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	ErrUnavailable = errors.New("auth service unavailable")
	ErrTimeout     = errors.New("auth service timeout")
	ErrBadResponse = errors.New("invalid auth service response")
	ErrInternal    = errors.New("auth service internal error")
)

type Client struct {
	conn    *grpc.ClientConn
	auth    authv1.AuthServiceClient
	timeout time.Duration
}

func New(addr string, timeout time.Duration) (*Client, error) {

	if addr == "" {
		return nil, errors.New("auth gRPC address must be set")
	}
	if timeout <= 0 {
		return nil, errors.New("auth timeout must be positive")
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:    conn,
		auth:    authv1.NewAuthServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Me(ctx context.Context, sessionID string) (domain.User, error) {
	if sessionID == "" {
		return domain.User{}, domain.ErrSessionExpired
	}

	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if id := logger.RequestID(ctx); id != "" {
		callCtx = metadata.AppendToOutgoingContext(callCtx, "x-request-id", id)
	}

	resp, err := c.auth.CheckSession(callCtx, &authv1.CheckSessionRequest{
		SessionId: sessionID,
	})
	if err != nil {
		if ctx.Err() != nil {
			return domain.User{}, ctx.Err()
		}

		switch status.Code(err) {
		case codes.Unauthenticated:
			return domain.User{}, domain.ErrSessionExpired
		case codes.Unavailable:
			return domain.User{}, ErrUnavailable
		case codes.DeadlineExceeded:
			return domain.User{}, ErrTimeout
		case codes.Internal:
			return domain.User{}, ErrInternal
		default:
			return domain.User{}, ErrBadResponse
		}
	}

	if resp.GetUserId() <= 0 {
		return domain.User{}, ErrBadResponse
	}

	return domain.User{
		ID:            resp.GetUserId(),
		EmailVerified: resp.GetEmailVerified(),
	}, nil
}
