package crypto

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestGetEncryptedEnv(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "key")
	key := []byte("0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(keyFile, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := load(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, c.gcm.NonceSize())
	sealed := c.gcm.Seal(nonce, nonce, []byte("database-secret"), nil)

	t.Setenv("RELAY_CONFIG_KEY_FILE", keyFile)
	t.Setenv("TEST_SECRET", prefix+base64.RawStdEncoding.EncodeToString(sealed)+")")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	value, err := GetEncryptedEnv("TEST_SECRET")
	if err != nil {
		t.Fatal(err)
	}
	if value != "database-secret" {
		t.Fatalf("GetEncryptedEnv() = %q", value)
	}
}

func TestGetEncryptedEnvAllowsPlaintextWithoutKeyFile(t *testing.T) {
	t.Setenv("RELAY_CONFIG_KEY_FILE", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("TEST_SECRET", "plain-value")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	value, err := GetEncryptedEnv("TEST_SECRET")
	if err != nil {
		t.Fatal(err)
	}
	if value != "plain-value" {
		t.Fatalf("GetEncryptedEnv() = %q", value)
	}
}

func TestGetEncryptedEnvRejectsInvalidCiphertext(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "key")
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	if err := os.WriteFile(keyFile, []byte(key), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RELAY_CONFIG_KEY_FILE", keyFile)
	t.Setenv("TEST_SECRET", "ENC(invalid)")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := GetEncryptedEnv("TEST_SECRET"); err == nil {
		t.Fatal("expected invalid ciphertext to fail")
	}
}
