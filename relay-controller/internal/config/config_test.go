package config

import (
	"path/filepath"
	"strings"
	"testing"

	"relay-controller/common/crypto"
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
	t.Setenv("MGMT_SERVICE_URL", "")
	t.Setenv("MGMT_SERVER_NAME", "")
	t.Setenv("RELAY_DOMAIN", "")
	t.Setenv("RELAY_REGION", "")
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "")
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
	t.Setenv("MGMT_SERVICE_URL", "https://mgmt.example.com")
	t.Setenv("MGMT_SERVER_NAME", "huaweicloud.com")
	t.Setenv("MGMT_CLIENT_CERT_BASE64", "certificate")
	t.Setenv("MGMT_CLIENT_KEY_BASE64", "private-key")
	t.Setenv("MGMT_CLIENT_KEY_PASSWORD", "secret")
	t.Setenv("MGMT_CA_CERT_BASE64", "ca-certificate")
	t.Setenv("DATABASE_DSN", "relay:secret@tcp(database:3306)/relay_controller")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "10.0.0.1:9443" ||
		cfg.DatabaseDSN != "relay:secret@tcp(database:3306)/relay_controller" ||
		cfg.Management.URL != "https://mgmt.example.com" ||
		cfg.Relay.Domain != "relay.example.com" || cfg.Relay.RequestsPerMinute != 240 {
		t.Fatalf("relay overrides = %#v", cfg.Relay)
	}
	if cfg.Management.ServerName != "huaweicloud.com" || cfg.Management.ClientCertBase64 != "certificate" ||
		cfg.Management.ClientKeyBase64 != "private-key" ||
		cfg.Management.ClientKeyPassword != "secret" || cfg.Management.CACertBase64 != "ca-certificate" {
		t.Fatalf("management overrides = %#v", cfg.Management)
	}
}

func TestLoadReportsSecretDecryptionFailure(t *testing.T) {
	t.Setenv("RELAY_CONFIG_KEY_FILE", filepath.Join(t.TempDir(), "missing-key"))
	t.Setenv("DATABASE_DSN", "ENC(invalid)")
	if err := crypto.Init(); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_DSN") {
		t.Fatalf("Load() error = %v", err)
	}
}
