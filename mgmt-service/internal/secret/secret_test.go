package secret

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeyFileAndEncryption(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "config.key")
	if err := GenerateKeyFile(keyFile); err != nil {
		t.Fatalf("GenerateKeyFile() error = %v", err)
	}
	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode = %o", info.Mode().Perm())
	}

	codec, err := Load(keyFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	encrypted, err := codec.Encrypt([]byte("secret-value"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if !strings.HasPrefix(encrypted, "ENC(v1.") || strings.Count(encrypted, ".") != 2 {
		t.Fatalf("Encrypt() format = %q", encrypted)
	}
	decrypted, err := codec.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != "secret-value" {
		t.Fatalf("Decrypt() = %q", decrypted)
	}
}

func TestDecryptRejectsLegacyFormat(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "config.key")
	if err := GenerateKeyFile(keyFile); err != nil {
		t.Fatalf("GenerateKeyFile() error = %v", err)
	}
	codec, err := Load(keyFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, err := codec.Decrypt("ENC(c2luZ2xlLXNlZ21lbnQ)"); err == nil {
		t.Fatal("Decrypt() error = nil for legacy format")
	}
}

func TestDecryptRejectsModifiedValue(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "config.key")
	if err := GenerateKeyFile(keyFile); err != nil {
		t.Fatalf("GenerateKeyFile() error = %v", err)
	}
	codec, err := Load(keyFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	encrypted, err := codec.Encrypt([]byte("secret-value"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	index := len(prefix) + 8
	replacement := "A"
	if encrypted[index] == 'A' {
		replacement = "B"
	}
	modified := encrypted[:index] + replacement + encrypted[index+1:]
	if _, err := codec.Decrypt(modified); err == nil {
		t.Fatal("Decrypt() error = nil for modified value")
	}
}
