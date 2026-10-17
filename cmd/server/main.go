package main

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/config"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/csrf"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
)

const csrfTokenTTL = 24 * time.Hour

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	slog.SetDefault(logger.New(os.Stdout, cfg.LogLevel))

	userRepo := repository.NewUserRepository()
	sessionRepo := repository.NewSessionRepository()
	adRepo := repository.NewAdRepository(repository.SeedAds())

	authUsecase := usecase.NewAuthUsecase(userRepo, sessionRepo)
	adUsecase := usecase.NewAdUsecase(adRepo)

	authHandler := delivery.NewAuthHandler(authUsecase, cfg.CookieSecure)
	adHandler := delivery.NewAdHandler(adUsecase)
	csrfHandler := delivery.NewCSRFHandler(csrf.New(csrfSecret(cfg.CSRFSecret), csrfTokenTTL), cfg.CookieSecure)

	router := delivery.NewRouter(cfg.FrontendOrigin, authHandler, adHandler, csrfHandler)

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

func csrfSecret(fromEnv string) []byte {
	if fromEnv != "" {
		return []byte(fromEnv)
	}

	slog.Warn("CSRF_SECRET is not set, using a random secret: csrf tokens reset on restart")
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	return secret
}
