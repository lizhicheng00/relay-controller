package config

import (
	"fmt"
	"os"
	"strings"

	"mgmt-service/internal/secret"
)

const (
	defaultAddress = ":8443"
	defaultDogFile = "/opt/cloud/dog/beta"
	pigOmega       = "MGMT_CONFIG_PIG"
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
	var codec *secret.Codec
	for _, field := range fields {
		if !secret.IsEncrypted(*field.value) {
			continue
		}
		if codec == nil {
			var err error
			codec, err = secret.Load(
				valueOrDefault("MGMT_CONFIG_DOG_FILE", defaultDogFile),
				pigOmega,
			)
			if err != nil {
				return Config{}, err
			}
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
