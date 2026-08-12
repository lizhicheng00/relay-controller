package config

import (
	"fmt"
	"os"
	"strconv"

	"relay-controller/internal/secret"
)

type Config struct {
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
	requestsPerMinute, err := strconv.Atoi(os.Getenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE"))
	if err != nil || requestsPerMinute < 1 {
		return Config{}, fmt.Errorf("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE must be a positive integer")
	}
	key := os.Getenv(secret.KeyEnvironment)
	databasePassword, err := loadSecret("DATASOURCE_PASSWORD", key)
	if err != nil {
		return Config{}, err
	}
	jwtPrivateKey, err := loadSecret("RELAY_JWT_PRIVATE_KEY", key)
	if err != nil {
		return Config{}, err
	}
	keyStorePassword, err := loadSecret("SERVER_SSL_KEY_STORE_PASSWORD", key)
	if err != nil {
		return Config{}, err
	}
	trustStorePassword, err := loadSecret("SERVER_SSL_TRUST_STORE_PASSWORD", key)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Database: Database{
			URL:      os.Getenv("DATASOURCE_URL"),
			Username: os.Getenv("DATASOURCE_USERNAME"),
			Password: databasePassword,
		},
		Relay: Relay{
			Domain:            os.Getenv("RELAY_DOMAIN"),
			Region:            os.Getenv("RELAY_REGION"),
			RequestsPerMinute: requestsPerMinute,
			JWTPrivateKey:     jwtPrivateKey,
		},
		TLS: TLS{
			KeyStoreBase64:     os.Getenv("SERVER_SSL_KEY_STORE_BASE64"),
			KeyStorePassword:   keyStorePassword,
			TrustStoreBase64:   os.Getenv("SERVER_SSL_TRUST_STORE_BASE64"),
			TrustStorePassword: trustStorePassword,
		},
	}, nil
}

func loadSecret(name, key string) (string, error) {
	value, err := secret.Resolve(name, os.Getenv(name), key)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return value, nil
}
