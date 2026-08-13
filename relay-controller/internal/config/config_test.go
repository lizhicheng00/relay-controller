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
	t.Setenv("RELAY_DOMAIN", "")
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Relay.Domain != "myhuaweicloud.com" || cfg.Relay.RequestsPerMinute != 120 {
		t.Fatalf("relay defaults = %#v", cfg.Relay)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	t.Setenv("RELAY_DOMAIN", "relay.example.com")
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "240")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Relay.Domain != "relay.example.com" || cfg.Relay.RequestsPerMinute != 240 {
		t.Fatalf("relay overrides = %#v", cfg.Relay)
	}
}
