package secret

import (
	"os"
	"path/filepath"
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
	decrypted, err := codec.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != "secret-value" {
		t.Fatalf("Decrypt() = %q", decrypted)
	}
	first := codec.DeriveKey("default-api-key")
	second := codec.DeriveKey("default-api-key")
	other := codec.DeriveKey("configuration")
	if first != second || first == other {
		t.Fatal("derived keys are not stable and purpose-separated")
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
	replacement := "A"
	if encrypted[len(encrypted)-2] == 'A' {
		replacement = "B"
	}
	modified := encrypted[:len(encrypted)-2] + replacement + ")"
	if _, err := codec.Decrypt(modified); err == nil {
		t.Fatal("Decrypt() error = nil for modified value")
	}
}
