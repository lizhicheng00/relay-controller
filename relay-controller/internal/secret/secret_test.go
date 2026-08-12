package secret

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncryptResolve(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	encrypted, err := Encrypt("DATASOURCE_PASSWORD", "database-secret", key)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encrypted, "ENC(v1.") || strings.Contains(encrypted, "database-secret") {
		t.Fatalf("unexpected encrypted value: %s", encrypted)
	}
	resolved, err := Resolve("DATASOURCE_PASSWORD", encrypted, key)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "database-secret" {
		t.Fatalf("resolved value = %q", resolved)
	}
}

func TestResolveRejectsDifferentConfigurationName(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	encrypted, err := Encrypt("DATASOURCE_PASSWORD", "database-secret", key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve("RELAY_JWT_PRIVATE_KEY", encrypted, key); err == nil {
		t.Fatal("expected authentication failure")
	}
}

func TestResolveLeavesPlaintextUnchanged(t *testing.T) {
	resolved, err := Resolve("DATASOURCE_PASSWORD", "local-password", "")
	if err != nil || resolved != "local-password" {
		t.Fatalf("resolved = %q, err = %v", resolved, err)
	}
}
