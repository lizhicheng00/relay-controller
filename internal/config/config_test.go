package config

import "testing"

func TestLoadConfiguration(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("SERVER_PORT", "9443")
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "180")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9443 || cfg.Relay.RequestsPerMinute != 180 || cfg.Relay.Region != "region-a" {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}

func TestLoadRejectsEncryptedPlaceholder(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("DATASOURCE_PASSWORD", "ENC(ciphertext)")

	if _, err := Load(); err == nil {
		t.Fatal("expected encrypted value without a decryptor to fail")
	}
}

func TestLoadRequiresMutualTLSMaterial(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("SERVER_SSL_KEY_STORE_BASE64", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected missing key store to fail")
	}
}

func TestLoadRejectsEncryptedTLSSecret(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("SERVER_SSL_KEY_STORE_PASSWORD", "${ENC(ciphertext)}")

	if _, err := Load(); err == nil {
		t.Fatal("expected encrypted TLS secret without a decryptor to fail")
	}
}

func TestLoadRejectsInvalidRateLimit(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "0")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid rate limit to fail")
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DATASOURCE_URL", "jdbc:mariadb://127.0.0.1:3306/relay_controller")
	t.Setenv("DATASOURCE_USERNAME", "relay_controller")
	t.Setenv("DATASOURCE_PASSWORD", "secret")
	t.Setenv("RELAY_DOMAIN", "myhuaweicloud.com")
	t.Setenv("RELAY_REGION", "region-a")
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "120")
	t.Setenv("RELAY_JWT_PRIVATE_KEY", "key")
	t.Setenv("SERVER_SSL_KEY_STORE_BASE64", "key-store")
	t.Setenv("SERVER_SSL_KEY_STORE_PASSWORD", "secret")
	t.Setenv("SERVER_SSL_TRUST_STORE_BASE64", "trust-store")
	t.Setenv("SERVER_SSL_TRUST_STORE_PASSWORD", "secret")
}
