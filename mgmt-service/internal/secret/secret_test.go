package secret

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThreeComponentEncryption(t *testing.T) {
	dogFile, _, codec := testCodec(t)
	info, err := os.Stat(dogFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("dog file mode = %o", info.Mode().Perm())
	}

	encrypted, err := codec.Encrypt([]byte("secret-value"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encrypted, "ENC(v1.") || strings.Count(encrypted, ".") != 2 {
		t.Fatalf("Encrypt() format = %q", encrypted)
	}
	decrypted, err := codec.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "secret-value" {
		t.Fatalf("Decrypt() = %q", decrypted)
	}

	otherPig := testComponent(t)
	t.Setenv("TEST_OTHER_CONFIG_PIG", otherPig)
	otherCodec, err := Load(dogFile, "TEST_OTHER_CONFIG_PIG")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherCodec.Decrypt(encrypted); err == nil {
		t.Fatal("Decrypt() succeeded with a different pig component")
	}
	t.Setenv("TEST_MISSING_CONFIG_PIG", "")
	if _, err := Load(dogFile, "TEST_MISSING_CONFIG_PIG"); err == nil {
		t.Fatal("Load() succeeded without the pig component")
	}
}

func TestDecryptRejectsInvalidValues(t *testing.T) {
	_, _, codec := testCodec(t)
	for _, value := range []string{
		"ENC(c2luZ2xlLXNlZ21lbnQ)",
		"ENC(v2.bm9uY2U.Y2lwaGVydGV4dA)",
	} {
		if _, err := codec.Decrypt(value); err == nil {
			t.Fatalf("Decrypt(%q) error = nil", value)
		}
	}

	encrypted, err := codec.Encrypt([]byte("secret-value"))
	if err != nil {
		t.Fatal(err)
	}
	index := strings.LastIndex(encrypted, ".") + 1
	replacement := "A"
	if encrypted[index] == 'A' {
		replacement = "B"
	}
	modified := encrypted[:index] + replacement + encrypted[index+1:]
	if _, err := codec.Decrypt(modified); err == nil {
		t.Fatal("Decrypt() succeeded with modified ciphertext")
	}
}

func testCodec(t *testing.T) (string, string, *Codec) {
	t.Helper()
	dogFile := filepath.Join(t.TempDir(), "dog")
	if err := os.WriteFile(dogFile, []byte(testComponent(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	pig := testComponent(t)
	t.Setenv("TEST_CONFIG_PIG", pig)
	codec, err := Load(dogFile, "TEST_CONFIG_PIG")
	if err != nil {
		t.Fatal(err)
	}
	return dogFile, pig, codec
}

func testComponent(t *testing.T) string {
	t.Helper()
	component := make([]byte, componentSize)
	if _, err := rand.Read(component); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(component)
}
