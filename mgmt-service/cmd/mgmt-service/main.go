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

	"mgmt-service/internal/apikey"
	"mgmt-service/internal/config"
	"mgmt-service/internal/httpapi"
	"mgmt-service/internal/service"
	"mgmt-service/internal/session/redisstore"
	"mgmt-service/internal/store/mysqlstore"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("management service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	repository, err := mysqlstore.Open(cfg.DatabaseDSN)
	if err != nil {
		return err
	}
	defer repository.Close()
	sessions := redisstore.Open(cfg.RedisAddress, cfg.RedisPassword)
	defer sessions.Close()

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelStartup()
	if err := repository.Ping(startupContext); err != nil {
		return err
	}
	if err := sessions.Ping(startupContext); err != nil {
		return err
	}

	application := service.New(
		repository,
		sessions,
		apikey.NewCodec(cfg.APIKeyPepper),
		logger,
	)
	handler := httpapi.NewServer(
		application, []httpapi.Readiness{repository, sessions}, cfg.TrustedProxyToken, logger)
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownContext, stop := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("DevBridge management service started", "address", cfg.Address)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-shutdownContext.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			return err
		}
		if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	logger.Info("DevBridge management service stopped")
	return nil
}
