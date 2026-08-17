package config

import (
	"encoding/json"
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

type secretValues struct {
	DatabaseDSN        string `json:"database_dsn"`
	KeyStoreBase64     string `json:"server_ssl_key_store_base64"`
	KeyStorePassword   string `json:"server_ssl_key_store_password"`
	TrustStoreBase64   string `json:"server_ssl_trust_store_base64"`
	TrustStorePassword string `json:"server_ssl_trust_store_password"`
}

func Load() (Config, error) {
	values, err := loadSecretValues()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Address:     valueOrDefault("SERVER_ADDRESS", defaultAddress),
		DatabaseDSN: strings.TrimSpace(values.DatabaseDSN),
		TLS: TLS{
			KeyStoreBase64:     values.KeyStoreBase64,
			KeyStorePassword:   values.KeyStorePassword,
			TrustStoreBase64:   values.TrustStoreBase64,
			TrustStorePassword: values.TrustStorePassword,
		},
	}, nil
}

func loadSecretValues() (secretValues, error) {
	path := strings.TrimSpace(os.Getenv("MGMT_CONFIG_FILE"))
	if path == "" {
		return secretValues{
			DatabaseDSN:        os.Getenv("DATABASE_DSN"),
			KeyStoreBase64:     os.Getenv("SERVER_SSL_KEY_STORE_BASE64"),
			KeyStorePassword:   os.Getenv("SERVER_SSL_KEY_STORE_PASSWORD"),
			TrustStoreBase64:   os.Getenv("SERVER_SSL_TRUST_STORE_BASE64"),
			TrustStorePassword: os.Getenv("SERVER_SSL_TRUST_STORE_PASSWORD"),
		}, nil
	}

	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return secretValues{}, fmt.Errorf("read encrypted configuration: %w", err)
	}
	plaintext, err := decrypt(ciphertext, os.Getenv(masterKeyEnvironment))
	if err != nil {
		return secretValues{}, fmt.Errorf("decrypt configuration: %w", err)
	}
	var values secretValues
	if err := json.Unmarshal(plaintext, &values); err != nil {
		return secretValues{}, fmt.Errorf("decode encrypted configuration: %w", err)
	}
	return values, nil
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
