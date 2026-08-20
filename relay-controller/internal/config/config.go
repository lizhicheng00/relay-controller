package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultRelayDomain       = "myhuaweicloud.com"
	defaultRequestsPerMinute = 120
	defaultManagementURL     = "https://127.0.0.1:8444"
	defaultAddress           = "127.0.0.1:8443"
)

type Config struct {
	Address    string
	Database   Database
	Relay      Relay
	Management Management
}

type Database struct {
	URL      string
	Username string
	Password string
}

type Relay struct {
	Domain            string
	Region            string
	RequestsPerMinute int
	JWTPrivateKey     string
}

type Management struct {
	URL               string
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
	return Config{
		Address: valueOrDefault("SERVER_ADDRESS", defaultAddress),
		Database: Database{
			URL:      os.Getenv("DATASOURCE_URL"),
			Username: os.Getenv("DATASOURCE_USERNAME"),
			Password: os.Getenv("DATASOURCE_PASSWORD"),
		},
		Relay: Relay{
			Domain:            valueOrDefault("RELAY_DOMAIN", defaultRelayDomain),
			Region:            os.Getenv("RELAY_REGION"),
			RequestsPerMinute: requestsPerMinute,
			JWTPrivateKey:     os.Getenv("RELAY_JWT_PRIVATE_KEY"),
		},
		Management: Management{
			URL:               valueOrDefault("MGMT_SERVICE_URL", defaultManagementURL),
			ClientCertBase64:  strings.TrimSpace(os.Getenv("MGMT_CLIENT_CERT_BASE64")),
			ClientKeyBase64:   strings.TrimSpace(os.Getenv("MGMT_CLIENT_KEY_BASE64")),
			ClientKeyPassword: os.Getenv("MGMT_CLIENT_KEY_PASSWORD"),
			CACertBase64:      strings.TrimSpace(os.Getenv("MGMT_CA_CERT_BASE64")),
		},
	}, nil
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
