package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"relay-controller/common/crypto"
)

const (
	defaultRelayDomain       = "myhuaweicloud.com"
	defaultRelayRegion       = "cn-north-4"
	defaultRequestsPerMinute = 120
	defaultManagementURL     = "https://127.0.0.1:8444"
	defaultManagementName    = "mgmt.developer.myhuaweicloud.com"
	defaultAddress           = ":8443"
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
		Address: valueOrDefault("SERVER_ADDRESS", defaultAddress),
		Relay: Relay{
			Domain:            valueOrDefault("RELAY_DOMAIN", defaultRelayDomain),
			Region:            valueOrDefault("RELAY_REGION", defaultRelayRegion),
			RequestsPerMinute: requestsPerMinute,
		},
		Management: Management{
			URL:              valueOrDefault("MGMT_SERVICE_URL", defaultManagementURL),
			ServerName:       valueOrDefault("MGMT_SERVER_NAME", defaultManagementName),
			ClientCertBase64: strings.TrimSpace(os.Getenv("MGMT_CLIENT_CERT_BASE64")),
			CACertBase64:     strings.TrimSpace(os.Getenv("MGMT_CA_CERT_BASE64")),
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
	for _, field := range fields {
		value, err := crypto.GetEncryptedEnv(field.name)
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
