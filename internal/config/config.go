package config

import (
	"fmt"
	"os"
	"strconv"
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
	return Config{
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
	}, nil
}
