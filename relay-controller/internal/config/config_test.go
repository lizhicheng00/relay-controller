package config

import (
	"encoding/base64"
	"testing"

	"relay-controller/internal/secret"
)

func TestLoadRejectsInvalidRateLimit(t *testing.T) {
	for _, value := range []string{"", "0", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", value)
			if _, err := Load(); err == nil {
				t.Fatal("expected invalid rate limit to fail")
			}
		})
	}
}

func TestLoadReadsRateLimit(t *testing.T) {
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "120")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Relay.RequestsPerMinute != 120 {
		t.Fatalf("requests per minute = %d", cfg.Relay.RequestsPerMinute)
	}
}

func TestLoadDecryptsSensitiveConfiguration(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	t.Setenv(secret.KeyEnvironment, key)
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "120")
	values := map[string]string{
		"DATASOURCE_PASSWORD":             "database-secret",
		"RELAY_JWT_PRIVATE_KEY":           "jwt-secret",
		"SERVER_SSL_KEY_STORE_PASSWORD":   "key-store-secret",
		"SERVER_SSL_TRUST_STORE_PASSWORD": "trust-store-secret",
	}
	for name, plaintext := range values {
		encrypted, err := secret.Encrypt(name, plaintext, key)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv(name, encrypted)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.Password != values["DATASOURCE_PASSWORD"] ||
		cfg.Relay.JWTPrivateKey != values["RELAY_JWT_PRIVATE_KEY"] ||
		cfg.TLS.KeyStorePassword != values["SERVER_SSL_KEY_STORE_PASSWORD"] ||
		cfg.TLS.TrustStorePassword != values["SERVER_SSL_TRUST_STORE_PASSWORD"] {
		t.Fatal("sensitive configuration was not decrypted")
	}
}
