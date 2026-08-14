package config

import (
	"os"
	"strings"
)

const defaultAddress = ":8443"

type Config struct {
	Address           string
	DatabaseDSN       string
	TrustedProxyToken string
	TLS               TLS
}

type TLS struct {
	KeyStoreBase64     string
	KeyStorePassword   string
	TrustStoreBase64   string
	TrustStorePassword string
}

func Load() Config {
	return Config{
		Address:           valueOrDefault("SERVER_ADDRESS", defaultAddress),
		DatabaseDSN:       strings.TrimSpace(os.Getenv("DATABASE_DSN")),
		TrustedProxyToken: os.Getenv("IDENTITY_PROXY_TOKEN"),
		TLS: TLS{
			KeyStoreBase64:     os.Getenv("SERVER_SSL_KEY_STORE_BASE64"),
			KeyStorePassword:   os.Getenv("SERVER_SSL_KEY_STORE_PASSWORD"),
			TrustStoreBase64:   os.Getenv("SERVER_SSL_TRUST_STORE_BASE64"),
			TrustStorePassword: os.Getenv("SERVER_SSL_TRUST_STORE_PASSWORD"),
		},
	}
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
