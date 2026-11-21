package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"

	authclient "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/client/auth"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/config"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/csrf"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/platform/httpserver"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/repository"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	slog.SetDefault(logger.New(os.Stdout, cfg.LogLevel))
	if err := run(cfg); err != nil {
		slog.Error("ads service failed", "err", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	if cfg.CSRFSecret == "" {
		return errors.New("CSRF_SECRET must be set")
	}
	connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	pool, err := repository.NewPool(connectCtx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer func() { _ = pool.Close() }()
	listingPhotoStorage, err := repository.NewFileStorage(filepath.Join(cfg.UploadDir, "listings"))
	if err != nil {
		return err
	}
	checker, err := authclient.New(
		httpserver.Env("AUTH_GRPC_ADDR", "127.0.0.1:9091"),
		time.Second,
	)
	if err != nil {
		return err
	}
	defer checker.Close()

	session := delivery.NewSessionMiddleware(checker)
	userRepo := repository.NewUserRepository(pool)
	adRepo := repository.NewAdRepository(pool)
	addressRepo := repository.NewAddressRepository(pool)
	cityRepo := repository.NewCityRepository(pool)
	sellerRepo := repository.NewSellerRepository(pool)
	tx := repository.NewTransactor(pool)
	adUsecase := usecase.NewAdUsecase(adRepo)
	addressUsecase := usecase.NewAddressUsecase(addressRepo, userRepo, tx)
	cityUsecase := usecase.NewCityUsecase(cityRepo)
	sellerUsecase := usecase.NewSellerUsecase(sellerRepo)
	listingRepo := repository.NewListingRepository(pool)
	listingUsecase := usecase.NewListingUsecase(listingRepo, listingPhotoStorage, tx)
	moderation := usecase.NewModerationUsecase(repository.NewModerationRepository(pool), listingRepo, tx)
	cartRepo := repository.NewCartRepository(pool)
	prices := domain.DeliveryPrices{Courier: cfg.CourierPrice, PickupPoint: cfg.PickupPointPrice}
	cartUsecase := usecase.NewCartUsecase(cartRepo, tx, prices)
	orderUsecase := usecase.NewOrderUsecase(cartRepo, repository.NewOrderRepository(pool), addressRepo, userRepo, tx, prices)
	adHandler := delivery.NewAdHandler(adUsecase)
	listingHandler := delivery.NewListingHandler(listingUsecase)
	addressHandler := delivery.NewAddressHandler(addressUsecase)
	cityHandler := delivery.NewCityHandler(cityUsecase)
	sellerHandler := delivery.NewSellerHandler(sellerUsecase)
	cartHandler := delivery.NewCartHandler(cartUsecase)
	orderHandler := delivery.NewOrderHandler(orderUsecase, cfg.PublicURL, cfg.FrontendOrigin)
	csrfHandler := delivery.NewCSRFHandler(csrf.New([]byte(cfg.CSRFSecret), 24*time.Hour), cfg.CookieSecure)
	maintenanceCtx, stopMaintenance := context.WithCancel(context.Background())
	listingDone, orderDone := make(chan struct{}), make(chan struct{})
	go func() { defer close(listingDone); runListingMaintenance(maintenanceCtx, listingUsecase) }()
	go func() { defer close(orderDone); runOrderMaintenance(maintenanceCtx, orderUsecase) }()
	defer func() { stopMaintenance(); <-listingDone; <-orderDone }()
	router := delivery.NewAdsRouter(cfg.FrontendOrigin, cfg.UploadDir, session, adHandler, listingHandler, csrfHandler, cartHandler, addressHandler, cityHandler, sellerHandler, orderHandler, delivery.NewModerationHandler(moderation))
	return httpserver.Run("ads", httpserver.Env("ADS_PORT", "8082"), router)
}

const (
	listingMaintenanceInterval = time.Minute
	orderMaintenanceInterval   = 5 * time.Second
)

func runListingMaintenance(ctx context.Context, listings *usecase.ListingUsecase) {
	ticker := time.NewTicker(listingMaintenanceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := listings.RunMaintenance(ctx, now); err != nil {
				slog.ErrorContext(ctx, "listing maintenance", "err", err)
			}
		}
	}
}

func runOrderMaintenance(ctx context.Context, orders *usecase.OrderUsecase) {
	ticker := time.NewTicker(orderMaintenanceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := orders.RunMaintenance(ctx); err != nil {
				slog.ErrorContext(ctx, "order maintenance", "err", err)
			}
		}
	}
}
