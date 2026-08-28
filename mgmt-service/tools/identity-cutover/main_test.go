package main

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mgmt-service/internal/secret"
)

func TestConvert(t *testing.T) {
	codec := testCodec(t)
	input := "# legacy identities\nmain domain-1 user-1 ns-main\nsub domain-1 user-2 ns-sub-1\n"
	var output strings.Builder

	if err := convert(input, &output, codec); err != nil {
		t.Fatal(err)
	}
	result := output.String()
	if strings.Contains(result, "domain-1") || strings.Contains(result, "user-1") ||
		strings.Contains(result, "user-2") || strings.Count(result, "UNHEX('") != 4 ||
		!strings.Contains(result, "'ns-main'") || !strings.Contains(result, "'ns-sub-1'") {
		t.Fatalf("unexpected SQL:\n%s", result)
	}
}

func TestConvertRejectsInvalidLine(t *testing.T) {
	if err := convert("main domain user\n", &strings.Builder{}, testCodec(t)); err == nil {
		t.Fatal("convert accepted an invalid line")
	}
}

func testCodec(t *testing.T) *secret.Codec {
	t.Helper()
	dogFile := filepath.Join(t.TempDir(), "dog")
	if err := os.WriteFile(dogFile, []byte(testComponent(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	codec, err := secret.Load(dogFile, testComponent(t))
	if err != nil {
		t.Fatal(err)
	}
	return codec
}

func testComponent(t *testing.T) string {
	t.Helper()
	component := make([]byte, 32)
	if _, err := rand.Read(component); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(component)
}
