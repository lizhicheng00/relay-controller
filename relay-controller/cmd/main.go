package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"relay-controller/internal/config"
	"relay-controller/internal/httpapi"
	"relay-controller/internal/secret"
	"relay-controller/internal/security"
	"relay-controller/internal/service"
	"relay-controller/internal/store"
)

func main() {
	if len(os.Args) > 1 {
		if err := runCommand(os.Args[1:]); err != nil {
			slog.Error("command failed", "error", err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("relay controller stopped", "error", err)
		os.Exit(1)
	}
}

func runCommand(arguments []string) error {
	if len(arguments) != 2 || arguments[0] != "encrypt-secret" {
		return fmt.Errorf("usage: relay-controller encrypt-secret CONFIG_NAME")
	}
	plaintext, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read secret: %w", err)
	}
	encrypted, err := secret.Encrypt(arguments[1], string(plaintext), os.Getenv(secret.KeyEnvironment))
	if err != nil {
		return err
	}
	fmt.Println(encrypted)
	return nil
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	signer, err := security.NewJWTSigner(cfg.Relay.JWTPrivateKey)
	if err != nil {
		return err
	}
	tlsConfig, err := security.TLSConfig(cfg.TLS)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	database, err := store.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer func() {
		_ = database.Close()
	}()

	application := service.New(database, signer, cfg.Relay.Domain, cfg.Relay.Region, logger)
	limiter := httpapi.NewRateLimiter(cfg.Relay.RequestsPerMinute)
	server := &http.Server{
		Handler:           httpapi.New(application, logger, limiter),
		ErrorLog:          log.New(io.Discard, "", 0),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
		TLSConfig:         tlsConfig,
	}
	listener, err := net.Listen("tcp", ":8443")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	jobs := application.StartJobs(ctx)
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ServeTLS(listener, "", "")
	}()
	logger.Info("Relay Controller started", "region", cfg.Relay.Region)

	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr = fmt.Errorf("serve HTTP: %w", err)
		}
	}
	stop()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownContext)
	if shutdownErr != nil {
		shutdownErr = fmt.Errorf("shutdown HTTP server: %w", shutdownErr)
	}
	jobs.Wait()
	if err := errors.Join(serveErr, shutdownErr); err != nil {
		return err
	}
	logger.Info("Relay Controller stopped")
	return nil
}
