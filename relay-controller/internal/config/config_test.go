package config

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"relay-controller/internal/secret"
)

func TestLoadRejectsInvalidRateLimit(t *testing.T) {
	for _, value := range []string{"0", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", value)
			if _, err := Load(); err == nil {
				t.Fatal("expected invalid rate limit to fail")
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SERVER_ADDRESS", "")
	t.Setenv("SERVER_SSL_CERT_BASE64", "")
	t.Setenv("SERVER_SSL_KEY_BASE64", "")
	t.Setenv("SERVER_SSL_KEY_PASSWORD", "")
	t.Setenv("MGMT_SERVICE_URL", "")
	t.Setenv("MGMT_SERVER_NAME", "")
	t.Setenv("RELAY_DOMAIN", "")
	t.Setenv("RELAY_REGION", "")
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "")
	t.Setenv("RELAY_TRUSTED_IDENTITY_TOKEN", "")
	t.Setenv("DATABASE_DSN", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != ":8443" || cfg.Management.URL != "https://127.0.0.1:8444" ||
		cfg.Management.ServerName != "mgmt.developer.myhuaweicloud.com" ||
		cfg.Relay.Domain != "myhuaweicloud.com" || cfg.Relay.Region != "cn-north-4" ||
		cfg.Relay.RequestsPerMinute != 120 {
		t.Fatalf("relay defaults = %#v", cfg.Relay)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	t.Setenv("RELAY_DOMAIN", "relay.example.com")
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "240")
	t.Setenv("SERVER_ADDRESS", "10.0.0.1:9443")
	t.Setenv("SERVER_SSL_CERT_BASE64", "server-certificate")
	t.Setenv("SERVER_SSL_KEY_BASE64", "server-key")
	t.Setenv("SERVER_SSL_KEY_PASSWORD", "server-password")
	t.Setenv("MGMT_SERVICE_URL", "https://mgmt.example.com")
	t.Setenv("MGMT_SERVER_NAME", "huaweicloud.com")
	t.Setenv("MGMT_CLIENT_CERT_BASE64", "certificate")
	t.Setenv("MGMT_CLIENT_KEY_BASE64", "private-key")
	t.Setenv("MGMT_CLIENT_KEY_PASSWORD", "secret")
	t.Setenv("MGMT_CA_CERT_BASE64", "ca-certificate")
	t.Setenv("RELAY_TRUSTED_IDENTITY_TOKEN", "identity-token")
	t.Setenv("DATABASE_DSN", "relay:secret@tcp(database:3306)/relay_controller")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "10.0.0.1:9443" ||
		cfg.DatabaseDSN != "relay:secret@tcp(database:3306)/relay_controller" ||
		cfg.Management.URL != "https://mgmt.example.com" ||
		cfg.Relay.Domain != "relay.example.com" || cfg.Relay.RequestsPerMinute != 240 ||
		cfg.Relay.TrustedIdentityToken != "identity-token" {
		t.Fatalf("relay overrides = %#v", cfg.Relay)
	}
	if cfg.TLS.CertificateBase64 != "server-certificate" || cfg.TLS.PrivateKeyBase64 != "server-key" ||
		cfg.TLS.PrivateKeyPassword != "server-password" {
		t.Fatalf("TLS overrides = %#v", cfg.TLS)
	}
	if cfg.Management.ServerName != "huaweicloud.com" || cfg.Management.ClientCertBase64 != "certificate" ||
		cfg.Management.ClientKeyBase64 != "private-key" ||
		cfg.Management.ClientKeyPassword != "secret" || cfg.Management.CACertBase64 != "ca-certificate" {
		t.Fatalf("management overrides = %#v", cfg.Management)
	}
}

func TestLoadReportsSecretDecryptionFailure(t *testing.T) {
	t.Setenv("RELAY_CONFIG_DOG_FILE", filepath.Join(t.TempDir(), "missing-dog"))
	t.Setenv("DATABASE_DSN", "ENC(invalid)")
	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestLoadDecryptsSecrets(t *testing.T) {
	dogFile := filepath.Join(t.TempDir(), "dog")
	if err := os.WriteFile(dogFile, []byte(testComponent(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	pig := testComponent(t)
	codec, err := secret.Load(dogFile, pig)
	if err != nil {
		t.Fatal(err)
	}
	dsn, err := codec.Encrypt([]byte("relay:password@tcp(database:3306)/relay"))
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := codec.Encrypt([]byte("private-key"))
	if err != nil {
		t.Fatal(err)
	}
	identityToken, err := codec.Encrypt([]byte("identity-token"))
	if err != nil {
		t.Fatal(err)
	}
	serverKey, err := codec.Encrypt([]byte("server-key"))
	if err != nil {
		t.Fatal(err)
	}
	serverKeyPassword, err := codec.Encrypt([]byte("server-password"))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("RELAY_CONFIG_DOG_FILE", dogFile)
	t.Setenv("omega", pig)
	t.Setenv("DATABASE_DSN", dsn)
	t.Setenv("RELAY_JWT_PRIVATE_KEY", privateKey)
	t.Setenv("RELAY_TRUSTED_IDENTITY_TOKEN", identityToken)
	t.Setenv("SERVER_SSL_KEY_BASE64", serverKey)
	t.Setenv("SERVER_SSL_KEY_PASSWORD", serverKeyPassword)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseDSN != "relay:password@tcp(database:3306)/relay" ||
		cfg.Relay.JWTPrivateKey != "private-key" || cfg.Relay.TrustedIdentityToken != "identity-token" ||
		cfg.TLS.PrivateKeyBase64 != "server-key" || cfg.TLS.PrivateKeyPassword != "server-password" {
		t.Fatalf("Load() = %#v", cfg)
	}
}

func testComponent(t *testing.T) string {
	t.Helper()
	component := make([]byte, 32)
	if _, err := rand.Read(component); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(component)
}
