package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("SERVER_ADDRESS", "")
	t.Setenv("SERVER_SSL_KEY_STORE_BASE64", "key-store")
	t.Setenv("SERVER_SSL_KEY_STORE_PASSWORD", "key-password")
	t.Setenv("SERVER_SSL_TRUST_STORE_BASE64", "trust-store")
	t.Setenv("SERVER_SSL_TRUST_STORE_PASSWORD", "trust-password")

	cfg := Load()
	if cfg.Address != ":8443" || cfg.TLS.KeyStoreBase64 != "key-store" ||
		cfg.TLS.KeyStorePassword != "key-password" || cfg.TLS.TrustStoreBase64 != "trust-store" ||
		cfg.TLS.TrustStorePassword != "trust-password" {
		t.Fatalf("Load() defaults = %#v", cfg)
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_DSN", "user:password@tcp(localhost:3306)/mgmt")
}
