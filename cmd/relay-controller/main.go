package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lizhicheng00/relay-controller/internal/config"
	"github.com/lizhicheng00/relay-controller/internal/httpapi"
	"github.com/lizhicheng00/relay-controller/internal/security"
	"github.com/lizhicheng00/relay-controller/internal/service"
	"github.com/lizhicheng00/relay-controller/internal/store"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("relay controller stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	signer, err := security.NewJWTSigner(
		cfg.Relay.JWTPrivateKey, cfg.Relay.JWTIssuer, cfg.Relay.JWTAudience,
		cfg.Relay.JWTKeyID, cfg.Relay.JWTTokenTTL)
	if err != nil {
		return err
	}
	var tlsConfig *tls.Config
	if cfg.TLS.Enabled {
		tlsConfig, err = security.TLSConfig(cfg.TLS)
		if err != nil {
			return err
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	database, err := store.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := database.Migrate(ctx); err != nil {
		return err
	}

	application := service.New(database, signer, cfg.Relay, logger)
	limiter := httpapi.NewRateLimiter(cfg.Relay.RateLimitEnabled, cfg.Relay.RequestsPerMinute)
	server := &http.Server{
		Handler:           httpapi.New(application, logger, limiter),
		ErrorLog:          log.New(serverLogWriter{logger: logger}, "", 0),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
		TLSConfig:         tlsConfig,
	}
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", cfg.Port, err)
	}
	jobs := application.StartJobs(ctx)
	serverErrors := make(chan error, 1)
	go func() {
		if cfg.TLS.Enabled {
			serverErrors <- server.ServeTLS(listener, "", "")
			return
		}
		serverErrors <- server.Serve(listener)
	}()
	protocol := "http"
	if cfg.TLS.Enabled {
		protocol = "https"
	}
	logger.Info("Relay Controller started", "version", version, "address", protocol+"://0.0.0.0:"+strconv.Itoa(cfg.Port), "region", cfg.Relay.Region)

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

type serverLogWriter struct {
	logger *slog.Logger
}

func (w serverLogWriter) Write(content []byte) (int, error) {
	message := strings.TrimSpace(string(content))
	if strings.Contains(message, "TLS handshake error") {
		w.logger.Debug("TLS client authentication rejected")
	} else {
		w.logger.Warn("HTTP server error", "message", message)
	}
	return len(content), nil
}
