package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mgmt-service/internal/config"
	"mgmt-service/internal/httpapi"
	"mgmt-service/internal/security"
	"mgmt-service/internal/service"
	"mgmt-service/internal/store"
	"mgmt-service/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Management Service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := migrations.Run(ctx, cfg.DatabaseDSN); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	repository, err := store.Open(ctx, cfg.DatabaseDSN)
	if err != nil {
		return err
	}
	defer func() { _ = repository.Close() }()
	application := service.New(repository, security.NewAPIKeys(cfg.APIKeySecret))
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           httpapi.New(application, cfg.TrustedProxyToken, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	logger.Info("Management Service started", "address", cfg.Address)

	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr = fmt.Errorf("serve HTTP: %w", err)
		}
	}
	stop()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownContext)
	if shutdownErr != nil {
		shutdownErr = fmt.Errorf("shutdown HTTP server: %w", shutdownErr)
	}
	if err := errors.Join(serveErr, shutdownErr); err != nil {
		return err
	}
	logger.Info("Management Service stopped")
	return nil
}
