package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("SERVER_ADDRESS", "")

	cfg := Load()
	if cfg.Address != ":8443" {
		t.Fatalf("Load() defaults = %#v", cfg)
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_DSN", "user:password@tcp(localhost:3306)/mgmt")
	t.Setenv("API_KEY_SECRET", strings.Repeat("s", 32))
	t.Setenv("IDENTITY_PROXY_TOKEN", strings.Repeat("t", 32))
}
