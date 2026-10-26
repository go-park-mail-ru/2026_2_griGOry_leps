package main

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/config"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/csrf"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/mail"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
)

const csrfTokenTTL = 24 * time.Hour

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	slog.SetDefault(logger.New(os.Stdout, cfg.LogLevel))

	connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	pool, err := repository.NewPool(connectCtx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		slog.Error("connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	slog.Info("connected to PostgreSQL")

	userRepo := repository.NewUserRepository(pool)
	sessionRepo := repository.NewSessionRepository(pool)
	adRepo := repository.NewAdRepository(pool)
	sellerRepo := repository.NewSellerRepository(pool)
	addressRepo := repository.NewAddressRepository(pool)
	cityRepo := repository.NewCityRepository(pool)

	avatarStorage, err := repository.NewFileStorage(filepath.Join(cfg.UploadDir, "avatars"))
	if err != nil {
		slog.Error("init avatar storage", "err", err)
		os.Exit(1)
	}

	mailer, err := newMailer(cfg)
	if err != nil {
		slog.Error("init mailer", "err", err)
		os.Exit(1)
	}

	tx := repository.NewTransactor(pool)
	authUsecase := usecase.NewAuthUsecase(userRepo, sessionRepo, tx)
	resetRepo := repository.NewPasswordResetRepository(pool)
	profileUsecase := usecase.NewProfileUsecase(userRepo, sessionRepo, resetRepo, avatarStorage, tx)
	resetUsecase := usecase.NewPasswordResetUsecase(userRepo, sessionRepo, resetRepo, mailer, tx, cfg.FrontendOrigin+"/reset-password")
	adUsecase := usecase.NewAdUsecase(adRepo, tx)
	sellerUsecase := usecase.NewSellerUsecase(sellerRepo)
	addressUsecase := usecase.NewAddressUsecase(addressRepo, userRepo, tx)
	cityUsecase := usecase.NewCityUsecase(cityRepo)

	authHandler := delivery.NewAuthHandler(authUsecase, cfg.CookieSecure)
	profileHandler := delivery.NewProfileHandler(profileUsecase)
	resetHandler := delivery.NewPasswordResetHandler(resetUsecase)
	adHandler := delivery.NewAdHandler(adUsecase)
	sellerHandler := delivery.NewSellerHandler(sellerUsecase)
	addressHandler := delivery.NewAddressHandler(addressUsecase)
	cityHandler := delivery.NewCityHandler(cityUsecase)
	csrfHandler := delivery.NewCSRFHandler(csrf.New(csrfSecret(cfg.CSRFSecret), csrfTokenTTL), cfg.CookieSecure)

	router := delivery.NewRouter(cfg.FrontendOrigin, cfg.UploadDir, authHandler, resetHandler, profileHandler, adHandler,
		csrfHandler, sellerHandler, addressHandler, cityHandler)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
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

func csrfSecret(fromEnv string) []byte {
	if fromEnv != "" {
		return []byte(fromEnv)
	}

	slog.Warn("CSRF_SECRET is not set, using a random secret: csrf tokens reset on restart")
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	return secret
}
