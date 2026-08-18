package config

import (
	"path/filepath"
	"testing"

	"mgmt-service/internal/secret"
)

func TestLoadPlainValues(t *testing.T) {
	setEnvironment(t)
	t.Setenv("MGMT_CONFIG_KEY_FILE", filepath.Join(t.TempDir(), "missing-key"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Address != ":8443" || cfg.DatabaseDSN != "plain-dsn" || !cfg.TLS.Enabled ||
		cfg.TLS.KeyStorePassword != "key-password" || cfg.TLS.TrustStorePassword != "trust-password" {
		t.Fatalf("Load() = %#v", cfg)
	}
}

func TestLoadWithoutTLS(t *testing.T) {
	setEnvironment(t)
	t.Setenv("SERVER_TLS_ENABLED", "false")
	t.Setenv("SERVER_SSL_KEY_STORE_PASSWORD", "ENC[unused]")
	t.Setenv("MGMT_CONFIG_KEY_FILE", filepath.Join(t.TempDir(), "missing-key"))

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TLS.Enabled {
		t.Fatal("TLS should be disabled")
	}
}

func TestLoadEncryptedValues(t *testing.T) {
	setEnvironment(t)
	keyFile, codec := testCodec(t)
	dsn, err := codec.Encrypt([]byte("encrypted-dsn"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	password, err := codec.Encrypt([]byte("encrypted-password"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	t.Setenv("MGMT_CONFIG_KEY_FILE", keyFile)
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

func TestLoadRejectsWrongKey(t *testing.T) {
	setEnvironment(t)
	_, codec := testCodec(t)
	value, err := codec.Encrypt([]byte("encrypted-dsn"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	wrongKeyFile, _ := testCodec(t)
	t.Setenv("DATABASE_DSN", value)
	t.Setenv("MGMT_CONFIG_KEY_FILE", wrongKeyFile)

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want decryption error")
	}
}

func setEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("SERVER_ADDRESS", "")
	t.Setenv("SERVER_TLS_ENABLED", "")
	t.Setenv("DATABASE_DSN", "plain-dsn")
	t.Setenv("SERVER_SSL_KEY_STORE_BASE64", "key-store")
	t.Setenv("SERVER_SSL_KEY_STORE_PASSWORD", "key-password")
	t.Setenv("SERVER_SSL_TRUST_STORE_BASE64", "trust-store")
	t.Setenv("SERVER_SSL_TRUST_STORE_PASSWORD", "trust-password")
}

func testCodec(t *testing.T) (string, *secret.Codec) {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "config.key")
	if err := secret.GenerateKeyFile(keyFile); err != nil {
		t.Fatalf("GenerateKeyFile() error = %v", err)
	}
	codec, err := secret.Load(keyFile)
	if err != nil {
		t.Fatalf("Load() key error = %v", err)
	}
	return keyFile, codec
}
