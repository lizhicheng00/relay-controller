package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultAddress       = ":8443"
	masterKeyEnvironment = "MGMT_CONFIG_MASTER_KEY"
)

type Config struct {
	Address     string
	DatabaseDSN string
	TLS         TLS
}

type TLS struct {
	KeyStoreBase64     string
	KeyStorePassword   string
	TrustStoreBase64   string
	TrustStorePassword string
}

func Load() (Config, error) {
	cfg := Config{
		Address:     valueOrDefault("SERVER_ADDRESS", defaultAddress),
		DatabaseDSN: os.Getenv("DATABASE_DSN"),
		TLS: TLS{
			KeyStoreBase64:     os.Getenv("SERVER_SSL_KEY_STORE_BASE64"),
			KeyStorePassword:   os.Getenv("SERVER_SSL_KEY_STORE_PASSWORD"),
			TrustStoreBase64:   os.Getenv("SERVER_SSL_TRUST_STORE_BASE64"),
			TrustStorePassword: os.Getenv("SERVER_SSL_TRUST_STORE_PASSWORD"),
		},
	}
	fields := []struct {
		name  string
		value *string
	}{
		{"DATABASE_DSN", &cfg.DatabaseDSN},
		{"SERVER_SSL_KEY_STORE_BASE64", &cfg.TLS.KeyStoreBase64},
		{"SERVER_SSL_KEY_STORE_PASSWORD", &cfg.TLS.KeyStorePassword},
		{"SERVER_SSL_TRUST_STORE_BASE64", &cfg.TLS.TrustStoreBase64},
		{"SERVER_SSL_TRUST_STORE_PASSWORD", &cfg.TLS.TrustStorePassword},
	}
	masterKey := os.Getenv(masterKeyEnvironment)
	for _, field := range fields {
		value, err := decryptIfEncrypted(*field.value, masterKey)
		if err != nil {
			return Config{}, fmt.Errorf("load %s: %w", field.name, err)
		}
		*field.value = value
	}
	cfg.DatabaseDSN = strings.TrimSpace(cfg.DatabaseDSN)
	return cfg, nil
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
