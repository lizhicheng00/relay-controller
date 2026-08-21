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
	return Config{
		Address: valueOrDefault("SERVER_ADDRESS", defaultAddress),
		Database: Database{
			URL:      os.Getenv("DATASOURCE_URL"),
			Username: os.Getenv("DATASOURCE_USERNAME"),
			Password: getSecret("DATASOURCE_PASSWORD"),
		},
		Relay: Relay{
			Domain:            valueOrDefault("RELAY_DOMAIN", defaultRelayDomain),
			Region:            valueOrDefault("RELAY_REGION", defaultRelayRegion),
			RequestsPerMinute: requestsPerMinute,
			JWTPrivateKey:     getSecret("RELAY_JWT_PRIVATE_KEY"),
		},
		Management: Management{
			URL:               valueOrDefault("MGMT_SERVICE_URL", defaultManagementURL),
			ServerName:        valueOrDefault("MGMT_SERVER_NAME", defaultManagementName),
			ClientCertBase64:  strings.TrimSpace(os.Getenv("MGMT_CLIENT_CERT_BASE64")),
			ClientKeyBase64:   strings.TrimSpace(getSecret("MGMT_CLIENT_KEY_BASE64")),
			ClientKeyPassword: getSecret("MGMT_CLIENT_KEY_PASSWORD"),
			CACertBase64:      strings.TrimSpace(os.Getenv("MGMT_CA_CERT_BASE64")),
		},
	}, nil
}

func getSecret(key string) string {
	value, err := crypto.GetEncryptedEnv(key)
	if err != nil {
		return ""
	}
	return value
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
