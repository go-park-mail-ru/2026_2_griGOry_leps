package mail

import (
	"context"
	"log/slog"
)

type Log struct{}

func (Log) SendPasswordReset(ctx context.Context, to, link string) error {
	slog.InfoContext(ctx, "password reset link", "to", to, "link", link)
	return nil
}
