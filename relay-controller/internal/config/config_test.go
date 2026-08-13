package config

import "testing"

func TestLoadRejectsInvalidRateLimit(t *testing.T) {
	for _, value := range []string{"", "0", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", value)
			if _, err := Load(); err == nil {
				t.Fatal("expected invalid rate limit to fail")
			}
		})
	}
}

func TestLoadReadsRateLimit(t *testing.T) {
	t.Setenv("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", "120")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Relay.RequestsPerMinute != 120 {
		t.Fatalf("requests per minute = %d", cfg.Relay.RequestsPerMinute)
	}
}
