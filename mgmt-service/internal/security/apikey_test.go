package security

import (
	"bytes"
	"strings"
	"testing"

	"mgmt-service/internal/core"
)

func TestNewAPIKeyIsRandomAndMasked(t *testing.T) {
	first, firstDigest := NewAPIKey(core.APIKeyScopeDevBox)
	second, _ := NewAPIKey(core.APIKeyScopeDevBox)
	devbridge, _ := NewAPIKey(core.APIKeyScopeDevBridge)
	if first == second || !strings.HasPrefix(first, "devbox_") || len(first) != len("devbox_")+32 {
		t.Fatalf("random API keys = %q, %q", first, second)
	}
	if !strings.HasPrefix(devbridge, "devbridge_") {
		t.Fatalf("invalid DevBridge API key = %q", devbridge)
	}
	scope, parsed, err := ParseAPIKey(first)
	if err != nil || scope != core.APIKeyScopeDevBox || !bytes.Equal(parsed, firstDigest) {
		t.Fatalf("ParseAPIKey() = %q, %x, %v", scope, parsed, err)
	}
	payload := strings.TrimPrefix(first, "devbox_")
	if mask := MaskAPIKey(first); mask != "devbox_"+payload[:4]+"..."+payload[len(payload)-4:] {
		t.Fatalf("MaskAPIKey() = %q", mask)
	}
}

func TestParseAPIKeyRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{
		"short",
		strings.Repeat("a", 32),
		"unknown_" + strings.Repeat("a", 32),
		"devbridge_" + strings.Repeat("+", 32),
	} {
		if _, _, err := ParseAPIKey(value); err == nil {
			t.Fatalf("ParseAPIKey(%q) succeeded", value)
		}
	}
}
