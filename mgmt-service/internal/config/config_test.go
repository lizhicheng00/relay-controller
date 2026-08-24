package config

import (
	"path/filepath"
	"testing"

	"mgmt-service/internal/secret"
)

func TestLoadPlainValues(t *testing.T) {
	setEnvironment(t)
	t.Setenv("MGMT_CONFIG_DOG_FILE", filepath.Join(t.TempDir(), "missing-dog"))

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
	dogFile, pig, codec := testCodec(t)
	dsn, err := codec.Encrypt([]byte("encrypted-dsn"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	password, err := codec.Encrypt([]byte("encrypted-password"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	t.Setenv("MGMT_CONFIG_DOG_FILE", dogFile)
	t.Setenv("MGMT_CONFIG_PIG", pig)
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
	_, pig, codec := testCodec(t)
	value, err := codec.Encrypt([]byte("encrypted-dsn"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	wrongDogFile, _, _ := testCodec(t)
	t.Setenv("DATABASE_DSN", value)
	t.Setenv("MGMT_CONFIG_DOG_FILE", wrongDogFile)
	t.Setenv("MGMT_CONFIG_PIG", pig)

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want decryption error")
	}
}

func setEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("SERVER_ADDRESS", "")
	t.Setenv("DATABASE_DSN", "plain-dsn")
	t.Setenv("SERVER_SSL_KEY_STORE_BASE64", "key-store")
	t.Setenv("SERVER_SSL_KEY_STORE_PASSWORD", "key-password")
	t.Setenv("SERVER_SSL_TRUST_STORE_BASE64", "trust-store")
	t.Setenv("SERVER_SSL_TRUST_STORE_PASSWORD", "trust-password")
}

func testCodec(t *testing.T) (string, string, *secret.Codec) {
	t.Helper()
	dogFile := filepath.Join(t.TempDir(), "dog")
	if err := secret.GenerateDogFile(dogFile); err != nil {
		t.Fatalf("GenerateDogFile() error = %v", err)
	}
	pig, err := secret.GenerateComponent()
	if err != nil {
		t.Fatalf("GenerateComponent() error = %v", err)
	}
	codec, err := secret.Load(dogFile, pig)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return dogFile, pig, codec
}
