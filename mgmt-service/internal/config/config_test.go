package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("SERVER_ADDRESS", "")
	t.Setenv("SERVER_SSL_KEY_STORE_BASE64", "key-store")
	t.Setenv("SERVER_SSL_KEY_STORE_PASSWORD", "key-password")
	t.Setenv("SERVER_SSL_TRUST_STORE_BASE64", "trust-store")
	t.Setenv("SERVER_SSL_TRUST_STORE_PASSWORD", "trust-password")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Address != ":8443" || cfg.TLS.KeyStoreBase64 != "key-store" ||
		cfg.TLS.KeyStorePassword != "key-password" || cfg.TLS.TrustStoreBase64 != "trust-store" ||
		cfg.TLS.TrustStorePassword != "trust-password" {
		t.Fatalf("Load() defaults = %#v", cfg)
	}
}

func TestLoadEncryptedConfiguration(t *testing.T) {
	key := testMasterKey()
	plaintext := []byte(`{
  "database_dsn": "encrypted-dsn",
  "server_ssl_key_store_base64": "encrypted-key-store",
  "server_ssl_key_store_password": "encrypted-key-password",
  "server_ssl_trust_store_base64": "encrypted-trust-store",
  "server_ssl_trust_store_password": "encrypted-trust-password"
}`)
	ciphertext, err := encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "mgmt-secrets.enc")
	if err := os.WriteFile(path, ciphertext, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("MGMT_CONFIG_FILE", path)
	t.Setenv("MGMT_CONFIG_MASTER_KEY", key)
	t.Setenv("DATABASE_DSN", "ignored-dsn")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DatabaseDSN != "encrypted-dsn" || cfg.TLS.KeyStoreBase64 != "encrypted-key-store" ||
		cfg.TLS.TrustStorePassword != "encrypted-trust-password" {
		t.Fatalf("Load() encrypted configuration = %#v", cfg)
	}
}

func TestLoadRejectsWrongMasterKey(t *testing.T) {
	ciphertext, err := encrypt([]byte(`{"database_dsn":"dsn"}`), testMasterKey())
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "mgmt-secrets.enc")
	if err := os.WriteFile(path, ciphertext, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("MGMT_CONFIG_FILE", path)
	t.Setenv("MGMT_CONFIG_MASTER_KEY", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32))))

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want decryption error")
	}
}

func TestGenerateMasterKey(t *testing.T) {
	encoded, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey() error = %v", err)
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		t.Fatalf("GenerateMasterKey() produced invalid key")
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("MGMT_CONFIG_FILE", "")
	t.Setenv("MGMT_CONFIG_MASTER_KEY", "")
	t.Setenv("DATABASE_DSN", "user:password@tcp(localhost:3306)/mgmt")
}

func testMasterKey() string {
	return base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
}
