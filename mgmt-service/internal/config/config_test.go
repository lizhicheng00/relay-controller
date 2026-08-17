package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadPlainValues(t *testing.T) {
	setEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Address != ":8443" || cfg.DatabaseDSN != "plain-dsn" ||
		cfg.TLS.KeyStorePassword != "key-password" || cfg.TLS.TrustStorePassword != "trust-password" {
		t.Fatalf("Load() = %#v", cfg)
	}
}

func TestLoadEncryptedValues(t *testing.T) {
	setEnvironment(t)
	key := testMasterKey("k")
	dsn, err := EncryptValue("encrypted-dsn", key)
	if err != nil {
		t.Fatalf("EncryptValue() error = %v", err)
	}
	password, err := EncryptValue("encrypted-password", key)
	if err != nil {
		t.Fatalf("EncryptValue() error = %v", err)
	}
	t.Setenv("MGMT_CONFIG_MASTER_KEY", key)
	t.Setenv("DATABASE_DSN", dsn)
	t.Setenv("SERVER_SSL_KEY_STORE_PASSWORD", password)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DatabaseDSN != "encrypted-dsn" || cfg.TLS.KeyStorePassword != "encrypted-password" ||
		cfg.TLS.TrustStorePassword != "trust-password" {
		t.Fatalf("Load() = %#v", cfg)
	}
}

func TestLoadRejectsWrongMasterKey(t *testing.T) {
	setEnvironment(t)
	value, err := EncryptValue("encrypted-dsn", testMasterKey("k"))
	if err != nil {
		t.Fatalf("EncryptValue() error = %v", err)
	}
	t.Setenv("DATABASE_DSN", value)
	t.Setenv("MGMT_CONFIG_MASTER_KEY", testMasterKey("x"))

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
		t.Fatal("GenerateMasterKey() produced invalid key")
	}
}

func setEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("MGMT_CONFIG_MASTER_KEY", "")
	t.Setenv("SERVER_ADDRESS", "")
	t.Setenv("DATABASE_DSN", "plain-dsn")
	t.Setenv("SERVER_SSL_KEY_STORE_BASE64", "key-store")
	t.Setenv("SERVER_SSL_KEY_STORE_PASSWORD", "key-password")
	t.Setenv("SERVER_SSL_TRUST_STORE_BASE64", "trust-store")
	t.Setenv("SERVER_SSL_TRUST_STORE_PASSWORD", "trust-password")
}

func testMasterKey(character string) string {
	return base64.StdEncoding.EncodeToString([]byte(strings.Repeat(character, 32)))
}
