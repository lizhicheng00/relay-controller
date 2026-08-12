package security

import (
	"bytes"
	"strings"
	"testing"
)

func TestCodecGeneratesBase36Key(t *testing.T) {
	codec := NewAPIKeyCodec("01234567890123456789012345678901")
	seen := make(map[string]struct{})
	for range 100 {
		value, mask, digest, err := codec.Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if len(value) != 32 || strings.Trim(value, alphabet) != "" {
			t.Fatalf("Generate() value = %q", value)
		}
		if mask != value[:2]+"..."+value[28:] {
			t.Fatalf("Generate() mask = %q", mask)
		}
		if len(digest) != 32 || bytes.Contains(digest, []byte(value)) {
			t.Fatalf("Generate() digest is invalid")
		}
		if _, exists := seen[value]; exists {
			t.Fatalf("Generate() returned duplicate value %q", value)
		}
		seen[value] = struct{}{}
		parsed, err := codec.Digest(value)
		if err != nil || !bytes.Equal(parsed, digest) {
			t.Fatalf("Digest() = %x, %v; want %x", parsed, err, digest)
		}
	}
}

func TestCodecRejectsMalformedKeys(t *testing.T) {
	codec := NewAPIKeyCodec("01234567890123456789012345678901")
	for _, value := range []string{
		"", strings.Repeat("a", 31), strings.Repeat("a", 33),
		strings.Repeat("A", 32), strings.Repeat("-", 32),
	} {
		if _, err := codec.Digest(value); err == nil {
			t.Errorf("Digest(%q) succeeded", value)
		}
	}
}

func TestCodecPepperSeparatesDigests(t *testing.T) {
	value := strings.Repeat("a", 32)
	first, _ := NewAPIKeyCodec(strings.Repeat("1", 32)).Digest(value)
	second, _ := NewAPIKeyCodec(strings.Repeat("2", 32)).Digest(value)
	if bytes.Equal(first, second) {
		t.Fatal("different peppers produced the same digest")
	}
}
