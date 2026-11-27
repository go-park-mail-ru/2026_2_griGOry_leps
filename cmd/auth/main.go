package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"

	adsclient "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/client/ads"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/config"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/csrf"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	authv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/auth/v1"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/mail"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/platform/httpserver"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/platform/metrics"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	slog.SetDefault(logger.New(os.Stdout, cfg.LogLevel))
	if err := run(cfg); err != nil {
		slog.Error("auth service failed", "err", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	if cfg.CSRFSecret == "" {
		return errors.New("CSRF_SECRET must be set")
	}
	connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	pool, err := repository.NewPool(connectCtx, cfg.AuthDatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer func() { _ = pool.Close() }()
	avatarStorage, err := repository.NewFileStorage(filepath.Join(cfg.UploadDir, "avatars"))
	if err != nil {
		return err
	}
	mailer, err := newMailer(cfg)
	if err != nil {
		return err
	}
	adsStats, err := adsclient.New(httpserver.Env("ADS_GRPC_ADDR", "127.0.0.1:9092"), time.Second)
	if err != nil {
		return err
	}
	defer adsStats.Close()
	userRepo := repository.NewUserRepository(pool)
	sessionRepo := repository.NewSessionRepository(pool)
	tx := repository.NewTransactor(pool)
	resetRepo := repository.NewPasswordResetRepository(pool)
	verifyRepo := repository.NewEmailVerificationRepository(pool)
	verifyUsecase := usecase.NewEmailVerificationUsecase(userRepo, verifyRepo, resetRepo, mailer, tx, cfg.FrontendOrigin+"/verify-email")
	authUsecase := usecase.NewAuthUsecase(userRepo, sessionRepo, verifyUsecase, tx)
	profileUsecase := usecase.NewProfileUsecase(userRepo, sessionRepo, resetRepo, verifyUsecase, avatarStorage, adsStats, tx)
	resetUsecase := usecase.NewPasswordResetUsecase(userRepo, sessionRepo, resetRepo, verifyUsecase, mailer, tx, cfg.FrontendOrigin+"/reset-password")
	authHandler := delivery.NewAuthHandler(authUsecase, cfg.CookieSecure)
	profileHandler := delivery.NewProfileHandler(profileUsecase)
	resetHandler := delivery.NewPasswordResetHandler(resetUsecase)
	verifyHandler := delivery.NewEmailVerificationHandler(verifyUsecase)
	cityHandler := delivery.NewCityHandler(usecase.NewCityUsecase(repository.NewCityRepository(pool)))
	csrfHandler := delivery.NewCSRFHandler(csrf.New([]byte(cfg.CSRFSecret), 24*time.Hour), cfg.CookieSecure)
	router := delivery.NewAuthRouter(cfg.FrontendOrigin, cfg.UploadDir, authHandler, csrfHandler, profileHandler, resetHandler, verifyHandler, cityHandler)
	listener, err := net.Listen("tcp", httpserver.Env("AUTH_GRPC_ADDR", "127.0.0.1:9091"))
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(metrics.RecordGRPCRequest),
	)
	authv1.RegisterAuthServiceServer(grpcServer, delivery.NewAuthGRPCServer(authUsecase, authUsecase))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	httpDone := make(chan error, 1)
	grpcDone := make(chan error, 1)
	go func() {
		httpDone <- httpserver.RunContext(ctx, "auth", "127.0.0.1", httpserver.Env("AUTH_PORT", "8081"), router)
		stop()
	}()
	go func() {
		slog.Info("auth gRPC starting", "addr", listener.Addr().String())
		grpcDone <- grpcServer.Serve(listener)
		stop()
	}()

	<-ctx.Done()
	timer := time.AfterFunc(10*time.Second, grpcServer.Stop)
	grpcServer.GracefulStop()
	timer.Stop()
	grpcErr := <-grpcDone
	if errors.Is(grpcErr, grpc.ErrServerStopped) {
		grpcErr = nil
	}
	return errors.Join(<-httpDone, grpcErr)
}

func newMailer(cfg *config.Config) (usecase.Mailer, error) {
	if cfg.SMTPHost == "" {
		slog.Warn("SMTP_HOST is not set, password reset links are written to the log")
		return mail.Log{}, nil
	}
	return mail.NewSMTP(mail.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		User:     cfg.SMTPUser,
		Password: cfg.SMTPPassword,
		From:     cfg.MailFrom,
	})
}
