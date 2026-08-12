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
	RedisAddress      string
	RedisPassword     string
	APIKeyPepper      string
	TrustedProxyToken string
}

func Load() (Config, error) {
	cfg := Config{
		Address:           valueOrDefault("SERVER_ADDRESS", defaultAddress),
		DatabaseDSN:       strings.TrimSpace(os.Getenv("DATABASE_DSN")),
		RedisAddress:      valueOrDefault("REDIS_ADDRESS", "localhost:6379"),
		RedisPassword:     os.Getenv("REDIS_PASSWORD"),
		APIKeyPepper:      os.Getenv("API_KEY_PEPPER"),
		TrustedProxyToken: os.Getenv("IAM_TRUSTED_PROXY_TOKEN"),
	}

	var missing []string
	if cfg.DatabaseDSN == "" {
		missing = append(missing, "DATABASE_DSN")
	}
	if len(cfg.APIKeyPepper) < 32 {
		missing = append(missing, "API_KEY_PEPPER (at least 32 characters)")
	}
	if len(cfg.TrustedProxyToken) < 32 {
		missing = append(missing, "IAM_TRUSTED_PROXY_TOKEN (at least 32 characters)")
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
