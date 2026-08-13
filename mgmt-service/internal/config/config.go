package config

import (
	"errors"
	"os"
	"strings"
)

const defaultAddress = ":8080"

type Config struct {
	Address           string
	DatabaseDSN       string
	APIKeySecret      string
	TrustedProxyToken string
}

func Load() (Config, error) {
	cfg := Config{
		Address:           valueOrDefault("SERVER_ADDRESS", defaultAddress),
		DatabaseDSN:       strings.TrimSpace(os.Getenv("DATABASE_DSN")),
		APIKeySecret:      os.Getenv("API_KEY_SECRET"),
		TrustedProxyToken: os.Getenv("IDENTITY_PROXY_TOKEN"),
	}

	var missing []string
	if cfg.DatabaseDSN == "" {
		missing = append(missing, "DATABASE_DSN")
	}
	if len(cfg.APIKeySecret) < 32 {
		missing = append(missing, "API_KEY_SECRET (at least 32 characters)")
	}
	if len(cfg.TrustedProxyToken) < 32 {
		missing = append(missing, "IDENTITY_PROXY_TOKEN (at least 32 characters)")
	}
	if len(missing) > 0 {
		return Config{}, errors.New("missing or invalid configuration: " + strings.Join(missing, ", "))
	}
	return cfg, nil
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
