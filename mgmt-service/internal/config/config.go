package config

import (
	"fmt"
	"os"
	"strings"

	"mgmt-service/internal/secret"
)

const (
	defaultAddress = ":8443"
	defaultKeyFile = "/opt/cloud/dog/beta"
)

type Config struct {
	Address      string
	DatabaseDSN  string
	APIKeyMaster [32]byte
	TLS          TLS
}

type TLS struct {
	KeyStoreBase64     string
	KeyStorePassword   string
	TrustStoreBase64   string
	TrustStorePassword string
}

func Load() (Config, error) {
	keyFile := valueOrDefault("MGMT_CONFIG_KEY_FILE", defaultKeyFile)
	codec, err := secret.Load(keyFile)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Address:      valueOrDefault("SERVER_ADDRESS", defaultAddress),
		DatabaseDSN:  os.Getenv("DATABASE_DSN"),
		APIKeyMaster: codec.DeriveKey("mgmt-default-api-key-v1"),
		TLS: TLS{
			KeyStoreBase64:     os.Getenv("SERVER_SSL_KEY_STORE_BASE64"),
			KeyStorePassword:   os.Getenv("SERVER_SSL_KEY_STORE_PASSWORD"),
			TrustStoreBase64:   os.Getenv("SERVER_SSL_TRUST_STORE_BASE64"),
			TrustStorePassword: os.Getenv("SERVER_SSL_TRUST_STORE_PASSWORD"),
		},
	}
	type field struct {
		name  string
		value *string
	}
	fields := []field{
		{"DATABASE_DSN", &cfg.DatabaseDSN},
		{"SERVER_SSL_KEY_STORE_BASE64", &cfg.TLS.KeyStoreBase64},
		{"SERVER_SSL_KEY_STORE_PASSWORD", &cfg.TLS.KeyStorePassword},
		{"SERVER_SSL_TRUST_STORE_BASE64", &cfg.TLS.TrustStoreBase64},
		{"SERVER_SSL_TRUST_STORE_PASSWORD", &cfg.TLS.TrustStorePassword},
	}
	for _, field := range fields {
		if !secret.IsEncrypted(*field.value) {
			continue
		}
		value, err := codec.Decrypt(*field.value)
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
