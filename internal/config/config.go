package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port     int
	Database Database
	Relay    Relay
	TLS      TLS
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

type TLS struct {
	KeyStoreBase64     string
	KeyStorePassword   string
	TrustStoreBase64   string
	TrustStorePassword string
}

func Load() (Config, error) {
	port, err := envInt("SERVER_PORT", 8443)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("SERVER_PORT must be between 1 and 65535")
	}
	requestsPerMinute, err := requiredInt("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE")
	if err != nil || requestsPerMinute < 1 {
		return Config{}, fmt.Errorf("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE must be a positive integer")
	}

	cfg := Config{
		Port: port,
		Database: Database{
			URL:      os.Getenv("DATASOURCE_URL"),
			Username: os.Getenv("DATASOURCE_USERNAME"),
			Password: os.Getenv("DATASOURCE_PASSWORD"),
		},
		Relay: Relay{
			Domain:            os.Getenv("RELAY_DOMAIN"),
			Region:            os.Getenv("RELAY_REGION"),
			RequestsPerMinute: requestsPerMinute,
			JWTPrivateKey:     os.Getenv("RELAY_JWT_PRIVATE_KEY"),
		},
		TLS: TLS{
			KeyStoreBase64:     os.Getenv("SERVER_SSL_KEY_STORE_BASE64"),
			KeyStorePassword:   os.Getenv("SERVER_SSL_KEY_STORE_PASSWORD"),
			TrustStoreBase64:   os.Getenv("SERVER_SSL_TRUST_STORE_BASE64"),
			TrustStorePassword: os.Getenv("SERVER_SSL_TRUST_STORE_PASSWORD"),
		},
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	required := [][2]string{
		{"DATASOURCE_URL", c.Database.URL},
		{"DATASOURCE_USERNAME", c.Database.Username},
		{"DATASOURCE_PASSWORD", c.Database.Password},
		{"RELAY_DOMAIN", c.Relay.Domain},
		{"RELAY_REGION", c.Relay.Region},
		{"RELAY_JWT_PRIVATE_KEY", c.Relay.JWTPrivateKey},
		{"SERVER_SSL_KEY_STORE_BASE64", c.TLS.KeyStoreBase64},
		{"SERVER_SSL_KEY_STORE_PASSWORD", c.TLS.KeyStorePassword},
		{"SERVER_SSL_TRUST_STORE_BASE64", c.TLS.TrustStoreBase64},
		{"SERVER_SSL_TRUST_STORE_PASSWORD", c.TLS.TrustStorePassword},
	}
	for _, item := range required {
		name, value := item[0], item[1]
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
		if looksEncrypted(value) {
			return fmt.Errorf("%s must be decrypted before it is injected", name)
		}
	}
	return nil
}

func envInt(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}

func requiredInt(name string) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}

func looksEncrypted(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "ENC(") || strings.HasPrefix(trimmed, "${ENC(")
}
