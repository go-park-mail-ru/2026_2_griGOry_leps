package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

var (
	ErrUnavailable = errors.New("auth service unavailable")
	ErrTimeout     = errors.New("auth service timeout")
	ErrBadResponse = errors.New("invalid auth service response")
)

type Client struct {
	sessionURL string
	httpClient *http.Client
}

func New(baseURL string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("invalid auth URL")
	}
	if timeout <= 0 {
		return nil, errors.New("auth timeout must be positive")
	}
	u.Path = "/internal/auth/session"
	return &Client{sessionURL: u.String(), httpClient: &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *Client) Me(ctx context.Context, sessionID string) (domain.User, error) {
	if sessionID == "" {
		return domain.User{}, domain.ErrSessionExpired
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.sessionURL, nil)
	if err != nil {
		return domain.User{}, err
	}
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.User{}, requestError(ctx, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return domain.User{}, domain.ErrSessionExpired
	case resp.StatusCode >= 500:
		return domain.User{}, ErrUnavailable
	case resp.StatusCode != http.StatusOK:
		return domain.User{}, ErrBadResponse
	}
	const maxBytes = 64 << 10
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return domain.User{}, ErrBadResponse
		}
		return domain.User{}, requestError(ctx, err)
	}
	if len(body) > maxBytes {
		return domain.User{}, ErrBadResponse
	}
	var result struct {
		UserID        int32 `json:"user_id"`
		EmailVerified *bool `json:"email_verified"`
	}
	if json.Unmarshal(body, &result) != nil || result.UserID <= 0 || result.EmailVerified == nil {
		return domain.User{}, ErrBadResponse
	}
	return domain.User{ID: result.UserID, EmailVerified: *result.EmailVerified}, nil
}

func requestError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	var timeoutError interface{ Timeout() bool }
	if errors.As(err, &timeoutError) && timeoutError.Timeout() {
		return ErrTimeout
	}
	return ErrUnavailable
}
