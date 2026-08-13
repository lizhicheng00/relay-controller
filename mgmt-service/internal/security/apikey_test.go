package security

import (
	"bytes"
	"strings"
	"testing"

	"mgmt-service/internal/core"
)

func TestAPIKeyIsStablePerIdentity(t *testing.T) {
	keys := NewAPIKeys(strings.Repeat("s", 32))
	first, firstDigest := keys.DefaultFor("domain-1", "user-1")
	second, secondDigest := keys.DefaultFor("domain-1", "user-1")
	other, _ := keys.DefaultFor("domain-1", "user-2")

	if first != second || !bytes.Equal(firstDigest, secondDigest) {
		t.Fatal("same identity produced different API keys")
	}
	if first == other || !strings.HasPrefix(first, "devbridge_") || len(first) != len("devbridge_")+32 {
		t.Fatalf("invalid API key %q", first)
	}
	parsed, err := DigestAPIKey(first)
	if err != nil || !bytes.Equal(parsed, firstDigest) {
		t.Fatalf("DigestAPIKey() = %x, %v", parsed, err)
	}
}

func TestNewAPIKeyIsRandomAndMasked(t *testing.T) {
	keys := NewAPIKeys(strings.Repeat("s", 32))
	first, firstDigest := keys.New(core.APIKeyScenarioDevBox)
	second, _ := keys.New(core.APIKeyScenarioDevBox)
	if first == second || !strings.HasPrefix(first, "devbox_") || len(first) != len("devbox_")+32 {
		t.Fatalf("random API keys = %q, %q", first, second)
	}
	parsed, err := DigestAPIKey(first)
	if err != nil || !bytes.Equal(parsed, firstDigest) {
		t.Fatalf("DigestAPIKey() = %x, %v", parsed, err)
	}
	payload := strings.TrimPrefix(first, "devbox_")
	if mask := MaskAPIKey(first); mask != "devbox_"+payload[:4]+"..."+payload[len(payload)-4:] {
		t.Fatalf("MaskAPIKey() = %q", mask)
	}
}

func TestDigestAPIKeyRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{
		"short",
		strings.Repeat("a", 32),
		"unknown_" + strings.Repeat("a", 32),
		"devbridge_" + strings.Repeat("+", 32),
	} {
		if _, err := DigestAPIKey(value); err == nil {
			t.Fatalf("DigestAPIKey(%q) succeeded", value)
		}
	}
}
