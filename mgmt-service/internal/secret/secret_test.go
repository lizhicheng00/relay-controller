package secret

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestThreeComponentEncryption(t *testing.T) {
	dogFile, _, codec := testCodec(t)
	info, err := os.Stat(dogFile)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
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
	otherCodec, err := Load(dogFile, otherPig)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherCodec.Decrypt(encrypted); err == nil {
		t.Fatal("Decrypt() succeeded with a different pig component")
	}
	if _, err := Load(dogFile, ""); err == nil {
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

func TestIdentityFingerprintIsStableAndScoped(t *testing.T) {
	dogFile, pig, codec := testCodec(t)
	reloaded, err := Load(dogFile, pig)
	if err != nil {
		t.Fatal(err)
	}
	domain := codec.Fingerprint("domain", "domain-1")
	sameDomain := reloaded.Fingerprint("domain", "domain-1")
	user := codec.Fingerprint("user", "domain-1", "user-1")
	otherDomainUser := codec.Fingerprint("user", "domain-2", "user-1")

	if len(domain) != 32 || !bytes.Equal(domain, sameDomain) {
		t.Fatalf("domain fingerprint is not a stable 32-byte digest")
	}
	if bytes.Equal(domain, user) || bytes.Equal(user, otherDomainUser) ||
		bytes.Contains(domain, []byte("domain-1")) || bytes.Contains(user, []byte("user-1")) {
		t.Fatal("identity fingerprints are not separated from their inputs")
	}
}

func testCodec(t *testing.T) (string, string, *Codec) {
	t.Helper()
	dogFile := filepath.Join(t.TempDir(), "dog")
	if err := os.WriteFile(dogFile, []byte(testComponent(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	pig := testComponent(t)
	codec, err := Load(dogFile, pig)
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
