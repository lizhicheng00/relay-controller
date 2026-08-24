package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"relay-controller/internal/secret"
)

const (
	defaultRelayDomain       = "myhuaweicloud.com"
	defaultRelayRegion       = "cn-north-4"
	defaultRequestsPerMinute = 120
	defaultManagementURL     = "https://127.0.0.1:8444"
	defaultManagementName    = "mgmt.developer.myhuaweicloud.com"
	defaultAddress           = ":8443"
	defaultKeyFile           = "/opt/cloud/dog/beta"
)

type Config struct {
	Address     string
	DatabaseDSN string
	Relay       Relay
	Management  Management
}

type Relay struct {
	Domain            string
	Region            string
	RequestsPerMinute int
	JWTPrivateKey     string
}

type Management struct {
	URL               string
	ServerName        string
	ClientCertBase64  string
	ClientKeyBase64   string
	ClientKeyPassword string
	CACertBase64      string
}

func Load() (Config, error) {
	requestsPerMinute := defaultRequestsPerMinute
	if value := strings.TrimSpace(os.Getenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return Config{}, fmt.Errorf("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE must be a positive integer")
		}
		requestsPerMinute = parsed
	}
	cfg := Config{
		Address:     valueOrDefault("SERVER_ADDRESS", defaultAddress),
		DatabaseDSN: os.Getenv("DATABASE_DSN"),
		Relay: Relay{
			Domain:            valueOrDefault("RELAY_DOMAIN", defaultRelayDomain),
			Region:            valueOrDefault("RELAY_REGION", defaultRelayRegion),
			RequestsPerMinute: requestsPerMinute,
			JWTPrivateKey:     os.Getenv("RELAY_JWT_PRIVATE_KEY"),
		},
		Management: Management{
			URL:               valueOrDefault("MGMT_SERVICE_URL", defaultManagementURL),
			ServerName:        valueOrDefault("MGMT_SERVER_NAME", defaultManagementName),
			ClientCertBase64:  strings.TrimSpace(os.Getenv("MGMT_CLIENT_CERT_BASE64")),
			ClientKeyBase64:   os.Getenv("MGMT_CLIENT_KEY_BASE64"),
			ClientKeyPassword: os.Getenv("MGMT_CLIENT_KEY_PASSWORD"),
			CACertBase64:      strings.TrimSpace(os.Getenv("MGMT_CA_CERT_BASE64")),
		},
	}
	fields := []struct {
		name  string
		value *string
	}{
		{"DATABASE_DSN", &cfg.DatabaseDSN},
		{"RELAY_JWT_PRIVATE_KEY", &cfg.Relay.JWTPrivateKey},
		{"MGMT_CLIENT_KEY_BASE64", &cfg.Management.ClientKeyBase64},
		{"MGMT_CLIENT_KEY_PASSWORD", &cfg.Management.ClientKeyPassword},
	}
	var codec *secret.Codec
	for _, field := range fields {
		if !secret.IsEncrypted(*field.value) {
			continue
		}
		if codec == nil {
			var err error
			codec, err = secret.Load(valueOrDefault("RELAY_CONFIG_KEY_FILE", defaultKeyFile))
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
	cfg.Management.ClientKeyBase64 = strings.TrimSpace(cfg.Management.ClientKeyBase64)
	return cfg, nil
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
