package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	authclient "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/client/auth"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/config"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/csrf"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/platform/httpserver"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	slog.SetDefault(logger.New(os.Stdout, cfg.LogLevel))
	if len(os.Args) > 1 {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		grant := func(ctx context.Context, id int32) (bool, error) {
			databaseURL, err := requireDatabaseURL(os.LookupEnv)
			if err != nil {
				return false, err
			}
			connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			pool, err := repository.NewPool(connectCtx, databaseURL)
			cancel()
			if err != nil {
				return false, err
			}
			defer func() { _ = pool.Close() }()
			admins := repository.NewOnlineConfAdminRepository(pool)
			return usecase.NewOnlineConfAccessUsecase(admins).GrantAdmin(ctx, id, nil)
		}
		code := runCLI(ctx, os.Args[1:], grant, os.Stdout, os.Stderr)
		stop()
		os.Exit(code)
	}
	if err := run(cfg); err != nil {
		slog.Error("onlineconf service failed", "err", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	if cfg.CSRFSecret == "" {
		return errors.New("CSRF_SECRET must be set")
	}
	connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := repository.NewPool(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = pool.Close() }()
	admins := repository.NewOnlineConfAdminRepository(pool)
	if err := admins.CheckReady(connectCtx); err != nil {
		return err
	}
	cancel()
	checker, err := authclient.New(httpserver.Env("AUTH_GRPC_ADDR", "127.0.0.1:9091"), time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = checker.Close() }()
	access := delivery.NewOnlineConfAccessHandler(usecase.NewOnlineConfAccessUsecase(admins))
	csrfHandler := delivery.NewCSRFHandler(csrf.New([]byte(cfg.CSRFSecret), 24*time.Hour), cfg.CookieSecure)
	admin := delivery.NewOnlineConfRouter(cfg.FrontendOrigin, delivery.NewSessionMiddleware(checker), csrfHandler, access)
	entries := delivery.NewOnlineconfRouter(usecase.NewOnlineconfUsecase(repository.NewOnlineconfRepository(pool)))
	// /v1/entries читает search по локальному адресу; gateway туда не проксирует, CSRF и сессия к нему не относятся.
	router := http.NewServeMux()
	router.Handle("/v1/", entries)
	router.Handle("/", admin)
	return httpserver.Run("onlineconf", "127.0.0.1", httpserver.Env("ONLINECONF_PORT", "8084"), router)
}
