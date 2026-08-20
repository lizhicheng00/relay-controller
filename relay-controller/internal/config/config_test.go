package config

import "testing"

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
	t.Setenv("RELAY_DOMAIN", "")
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "127.0.0.1:8443" || cfg.Management.URL != "https://127.0.0.1:8444" ||
		cfg.Relay.Domain != "myhuaweicloud.com" || cfg.Relay.RequestsPerMinute != 120 {
		t.Fatalf("relay defaults = %#v", cfg.Relay)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	t.Setenv("RELAY_DOMAIN", "relay.example.com")
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "240")
	t.Setenv("SERVER_ADDRESS", "10.0.0.1:9443")
	t.Setenv("MGMT_SERVICE_URL", "https://mgmt.example.com")
	t.Setenv("MGMT_CLIENT_CERT_FILE", "/run/secrets/client.crt")
	t.Setenv("MGMT_CLIENT_KEY_FILE", "/run/secrets/client.key")
	t.Setenv("MGMT_CLIENT_KEY_PASSWORD", "secret")
	t.Setenv("MGMT_CA_CERT_FILE", "/run/secrets/ca.crt")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "10.0.0.1:9443" || cfg.Management.URL != "https://mgmt.example.com" ||
		cfg.Relay.Domain != "relay.example.com" || cfg.Relay.RequestsPerMinute != 240 {
		t.Fatalf("relay overrides = %#v", cfg.Relay)
	}
	if cfg.Management.ClientCertFile != "/run/secrets/client.crt" ||
		cfg.Management.ClientKeyFile != "/run/secrets/client.key" ||
		cfg.Management.ClientKeyPassword != "secret" || cfg.Management.CACertFile != "/run/secrets/ca.crt" {
		t.Fatalf("management overrides = %#v", cfg.Management)
	}
}
