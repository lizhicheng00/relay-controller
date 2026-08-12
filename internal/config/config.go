package config

import (
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

func Load() Config {
	requestsPerMinute, _ := strconv.Atoi(os.Getenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE"))
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
	}
}
