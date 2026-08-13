package security

import (
	"bytes"
	"strings"
	"testing"
)

func TestAPIKeyIsStablePerIdentity(t *testing.T) {
	keys := NewAPIKeys(strings.Repeat("s", 32))
	first, firstDigest := keys.DefaultFor("domain-1", "user-1")
	second, secondDigest := keys.DefaultFor("domain-1", "user-1")
	other, _ := keys.DefaultFor("domain-1", "user-2")

	if first != second || !bytes.Equal(firstDigest, secondDigest) {
		t.Fatal("same identity produced different API keys")
	}
	if first == other || len(first) != apiKeyLength ||
		strings.Trim(first, "abcdefghijklmnopqrstuvwxyz0123456789") != "" {
		t.Fatalf("invalid API key %q", first)
	}
	parsed, err := DigestAPIKey(first)
	if err != nil || !bytes.Equal(parsed, firstDigest) {
		t.Fatalf("DigestAPIKey() = %x, %v", parsed, err)
	}
}

func TestNewAPIKeyIsRandomAndMasked(t *testing.T) {
	keys := NewAPIKeys(strings.Repeat("s", 32))
	first, firstDigest, err := keys.New()
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := keys.New()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(first) != apiKeyLength {
		t.Fatalf("random API keys = %q, %q", first, second)
	}
	parsed, err := DigestAPIKey(first)
	if err != nil || !bytes.Equal(parsed, firstDigest) {
		t.Fatalf("DigestAPIKey() = %x, %v", parsed, err)
	}
	if mask := MaskAPIKey(first); mask != first[:4]+"..."+first[len(first)-4:] {
		t.Fatalf("MaskAPIKey() = %q", mask)
	}
}

func TestDigestAPIKeyRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"short", strings.Repeat("A", apiKeyLength), strings.Repeat("-", apiKeyLength)} {
		if _, err := DigestAPIKey(value); err == nil {
			t.Fatalf("DigestAPIKey(%q) succeeded", value)
		}
	}
}
