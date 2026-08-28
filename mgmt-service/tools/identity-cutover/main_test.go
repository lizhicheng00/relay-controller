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
	namespaces := `[
		{"namespace":"ns-main","user_domain_id":"domain-1"},
		{"namespace":"ns-other","user_domain_id":"domain-2"}
	]`
	users := `[
		{"user_id":"user-2","customer_id":"domain-2"},
		{"user_id":"user-1","customer_id":"domain-1"}
	]`
	var output strings.Builder

	if err := convert(namespaces, users, &output, codec); err != nil {
		t.Fatal(err)
	}
	result := output.String()
	if strings.Contains(result, "domain-1") || strings.Contains(result, "user-1") ||
		strings.Contains(result, "user-2") || strings.Count(result, "UNHEX('") != 4 ||
		!strings.Contains(result, "'ns-main'") || !strings.Contains(result, "'ns-other'") ||
		strings.Count(result, "('main'") != 2 {
		t.Fatalf("unexpected SQL:\n%s", result)
	}
}

func TestConvertRejectsAmbiguousUserMapping(t *testing.T) {
	namespaces := `[{"namespace":"ns-main","user_domain_id":"domain-1"}]`
	users := `[
		{"user_id":"user-1","customer_id":"domain-1"},
		{"user_id":"user-2","customer_id":"domain-1"}
	]`
	if err := convert(namespaces, users, &strings.Builder{}, testCodec(t)); err == nil {
		t.Fatal("convert accepted an ambiguous user mapping")
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
