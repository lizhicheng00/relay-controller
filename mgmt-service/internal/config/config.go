package config

import (
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

func Load() Config {
	return Config{
		Address:           valueOrDefault("SERVER_ADDRESS", defaultAddress),
		DatabaseDSN:       strings.TrimSpace(os.Getenv("DATABASE_DSN")),
		APIKeySecret:      os.Getenv("API_KEY_SECRET"),
		TrustedProxyToken: os.Getenv("IDENTITY_PROXY_TOKEN"),
	}
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
