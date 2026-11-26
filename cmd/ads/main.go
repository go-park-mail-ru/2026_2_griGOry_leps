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

	authclient "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/client/auth"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/config"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/csrf"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/delivery"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	adsv1 "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/gen/ads/v1"
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
	pool, err := repository.NewPool(connectCtx, cfg.AdsDatabaseURL)
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
	adRepo := repository.NewAdRepository(pool)
	addressRepo := repository.NewAddressRepository(pool)
	sellerRepo := repository.NewSellerRepository(pool)
	tx := repository.NewTransactor(pool)
	adUsecase := usecase.NewAdUsecase(adRepo)
	addressUsecase := usecase.NewAddressUsecase(addressRepo, tx)
	sellerUsecase := usecase.NewSellerUsecase(sellerRepo, checker)
	listingRepo := repository.NewListingRepository(pool)
	listingUsecase := usecase.NewListingUsecase(listingRepo, listingPhotoStorage, checker, tx)
	moderation := usecase.NewModerationUsecase(repository.NewModerationRepository(pool), listingRepo, checker, tx)
	cartRepo := repository.NewCartRepository(pool)
	prices := domain.DeliveryPrices{Courier: cfg.CourierPrice, PickupPoint: cfg.PickupPointPrice}
	cartUsecase := usecase.NewCartUsecase(cartRepo, checker, tx, prices)
	orderUsecase := usecase.NewOrderUsecase(cartRepo, repository.NewOrderRepository(pool), addressRepo, checker, tx, prices)
	adHandler := delivery.NewAdHandler(adUsecase)
	listingHandler := delivery.NewListingHandler(listingUsecase)
	addressHandler := delivery.NewAddressHandler(addressUsecase)
	sellerHandler := delivery.NewSellerHandler(sellerUsecase)
	cartHandler := delivery.NewCartHandler(cartUsecase)
	orderHandler := delivery.NewOrderHandler(orderUsecase, cfg.PublicURL, cfg.FrontendOrigin)
	csrfHandler := delivery.NewCSRFHandler(csrf.New([]byte(cfg.CSRFSecret), 24*time.Hour), cfg.CookieSecure)
	maintenanceCtx, stopMaintenance := context.WithCancel(context.Background())
	listingDone, orderDone := make(chan struct{}), make(chan struct{})
	go func() { defer close(listingDone); runListingMaintenance(maintenanceCtx, listingUsecase) }()
	go func() { defer close(orderDone); runOrderMaintenance(maintenanceCtx, orderUsecase) }()
	defer func() { stopMaintenance(); <-listingDone; <-orderDone }()
	router := delivery.NewAdsRouter(cfg.FrontendOrigin, cfg.UploadDir, session, adHandler, listingHandler, csrfHandler, cartHandler, addressHandler, sellerHandler, orderHandler, delivery.NewModerationHandler(moderation))
	listener, err := net.Listen("tcp", httpserver.Env("ADS_GRPC_ADDR", "127.0.0.1:9092"))
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	adsv1.RegisterAdsServiceServer(grpcServer, delivery.NewAdsGRPCServer(adUsecase, sellerUsecase))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpDone := make(chan error, 1)
	grpcDone := make(chan error, 1)

	go func() {
		httpDone <- httpserver.RunContext(ctx, "ads", "127.0.0.1", httpserver.Env("ADS_PORT", "8082"), router)
		stop()
	}()

	go func() {
		slog.Info("ads gRPC starting", "addr", listener.Addr().String())
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
