package auth

import (
	"context"
	"errors"
	"slices"
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

// maxUsersBatch — лимит GetUsers на стороне auth.
const maxUsersBatch = 100

// callContext ограничивает вызов таймаутом клиента и передаёт request id в metadata.
func (c *Client) callContext(ctx context.Context) (context.Context, context.CancelFunc) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	if id := logger.RequestID(ctx); id != "" {
		callCtx = metadata.AppendToOutgoingContext(callCtx, "x-request-id", id)
	}
	return callCtx, cancel
}

// mapError переводит ошибку gRPC в ошибки пакета; ctx — контекст исходного запроса.
func mapError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	switch status.Code(err) {
	case codes.Unauthenticated:
		return domain.ErrSessionExpired
	case codes.Unavailable:
		return ErrUnavailable
	case codes.DeadlineExceeded:
		return ErrTimeout
	case codes.Internal:
		return ErrInternal
	default:
		return ErrBadResponse
	}
}

func (c *Client) Me(ctx context.Context, sessionID string) (domain.User, error) {
	if sessionID == "" {
		return domain.User{}, domain.ErrSessionExpired
	}

	callCtx, cancel := c.callContext(ctx)
	defer cancel()

	resp, err := c.auth.CheckSession(callCtx, &authv1.CheckSessionRequest{SessionId: sessionID})
	if err != nil {
		return domain.User{}, mapError(ctx, err)
	}
	if resp.GetUserId() <= 0 {
		return domain.User{}, ErrBadResponse
	}

	return domain.User{
		ID:            resp.GetUserId(),
		EmailVerified: resp.GetEmailVerified(),
		IsModerator:   resp.GetIsModerator(),
	}, nil
}

// Users возвращает карточки пользователей по ID. Неизвестные ID в результат не попадают.
// Повторы и неположительные ID отбрасываются, большие списки уходят пачками по maxUsersBatch.
func (c *Client) Users(ctx context.Context, ids []int32) (map[int32]domain.UserCard, error) {
	unique := make([]int32, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			unique = append(unique, id)
		}
	}
	slices.Sort(unique)
	unique = slices.Compact(unique)

	cards := make(map[int32]domain.UserCard, len(unique))
	for batch := range slices.Chunk(unique, maxUsersBatch) {
		if err := c.usersBatch(ctx, batch, cards); err != nil {
			return nil, err
		}
	}
	return cards, nil
}

func (c *Client) usersBatch(ctx context.Context, ids []int32, out map[int32]domain.UserCard) error {
	callCtx, cancel := c.callContext(ctx)
	defer cancel()

	resp, err := c.auth.GetUsers(callCtx, &authv1.GetUsersRequest{UserIds: ids})
	if err != nil {
		return mapError(ctx, err)
	}
	for _, u := range resp.GetUsers() {
		if !slices.Contains(ids, u.GetUserId()) {
			return ErrBadResponse
		}
		out[u.GetUserId()] = domain.UserCard{
			ID:            u.GetUserId(),
			FirstName:     u.GetFirstName(),
			Avatar:        u.GetAvatar(),
			City:          u.GetCity(),
			Phone:         u.GetPhone(),
			PhoneVerified: u.GetPhoneVerified(),
			CreatedAt:     time.UnixMicro(u.GetCreatedUnixMicros()).UTC(),
		}
	}
	return nil
}
