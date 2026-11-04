package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/laddawan/backend-test/api"
	"github.com/laddawan/backend-test/config"
	"github.com/laddawan/backend-test/database"
	"github.com/laddawan/backend-test/grpcapi"
	"github.com/laddawan/backend-test/logger"
	"github.com/laddawan/backend-test/repository/mongodb"
	"github.com/laddawan/backend-test/router"
	"github.com/laddawan/backend-test/security"
	"github.com/laddawan/backend-test/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log := logger.New(cfg.LogFormat)

	// ctx is cancelled on SIGINT/SIGTERM, which starts graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- Database ---------------------------------------------------------
	connectCtx, cancelConnect := context.WithTimeout(ctx, 15*time.Second)
	defer cancelConnect()

	client, err := database.Connect(connectCtx, cfg.MongoURI)
	if err != nil {
		return err
	}
	defer func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Disconnect(disconnectCtx); err != nil {
			log.Error("disconnect mongodb", "error", err)
		}
		log.Info("mongodb disconnected")
	}()
	db := client.Database(cfg.MongoDatabase)
	log.Info("connected to mongodb", "database", cfg.MongoDatabase)

	if err := database.Migrate(connectCtx, db); err != nil {
		return err
	}
	log.Info("database migrated")

	// --- Security -----------------------------------------------------------
	jwtManager, err := security.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	if err != nil {
		return err
	}
	hasher := security.NewBcryptHasher(cfg.BcryptCost)

	// --- Repository -> Usecase -> Handler ------------------------------------
	userRepository := mongodb.NewUserRepository(db)

	validator := usecase.NewValidator()
	userUsecase := usecase.NewUserUsecase(userRepository, hasher, validator)
	authUsecase, err := usecase.NewAuthUsecase(userUsecase, userRepository, hasher, jwtManager, validator)
	if err != nil {
		return err
	}

	authHandler := api.NewAuthHandler(authUsecase, log)
	userHandler := api.NewUserHandler(userUsecase, log)

	// --- HTTP (Fiber) -------------------------------------------------------
	app := router.NewApp(log)
	router.SetRouter(app, authHandler, userHandler, jwtManager)

	httpListener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen http: %w", err)
	}

	// --- gRPC ---------------------------------------------------------------
	grpcServer := grpcapi.NewServer(userUsecase, jwtManager, log)
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	// --- Background goroutine: log user count -------------------------------
	// Workers get their own context so they keep running until the servers
	// have drained, then stop explicitly during shutdown.
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		usecase.NewUserCounter(userRepository, cfg.UserCountInterval, log).Run(workerCtx)
	}()

	// --- Start servers ------------------------------------------------------
	serverErrs := make(chan error, 2)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := app.Listener(httpListener, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			serverErrs <- fmt.Errorf("http server: %w", err)
		}
	}()
	go func() {
		log.Info("grpc server listening", "addr", cfg.GRPCAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			serverErrs <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	// --- Wait for a signal or a fatal server error --------------------------
	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case runErr = <-serverErrs:
		log.Error("server failed, shutting down", "error", runErr)
	}

	shutdown(log, cfg.ShutdownTimeout, app, grpcServer.GracefulStop, grpcServer.Stop)

	stopWorkers()
	workers.Wait()

	// MongoDB is disconnected by the deferred call above, after all
	// in-flight requests and workers have finished.
	return runErr
}

// shutdown stops the HTTP server, then the gRPC server, within timeout.
func shutdown(log *slog.Logger, timeout time.Duration, app *fiber.App, grpcGracefulStop, grpcStop func()) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Error("http shutdown", "error", err)
	} else {
		log.Info("http server stopped")
	}

	stopped := make(chan struct{})
	go func() {
		grpcGracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
		log.Info("grpc server stopped")
	case <-ctx.Done():
		grpcStop()
		log.Warn("grpc server forced to stop after timeout")
	}
}
