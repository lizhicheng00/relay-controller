package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("SERVER_ADDRESS", "")
	t.Setenv("REDIS_ADDRESS", "")
	t.Setenv("REDIS_PASSWORD", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Address != ":8080" || cfg.RedisAddress != "localhost:6379" {
		t.Fatalf("Load() defaults = %#v", cfg)
	}
}

func TestLoadRequiresSecrets(t *testing.T) {
	t.Setenv("DATABASE_DSN", "")
	t.Setenv("API_KEY_PEPPER", "short")
	t.Setenv("IAM_TRUSTED_PROXY_TOKEN", "short")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded without required secrets")
	}
	for _, name := range []string{"DATABASE_DSN", "API_KEY_PEPPER", "IAM_TRUSTED_PROXY_TOKEN"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("Load() error %q does not mention %s", err, name)
		}
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_DSN", "user:password@tcp(localhost:3306)/mgmt")
	t.Setenv("API_KEY_PEPPER", strings.Repeat("p", 32))
	t.Setenv("IAM_TRUSTED_PROXY_TOKEN", strings.Repeat("t", 32))
}
