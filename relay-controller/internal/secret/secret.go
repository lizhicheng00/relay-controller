package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	prefix  = "ENC("
	version = "v1"
	keySize = 32
)

type Codec struct {
	gcm cipher.AEAD
}

func Load(keyFile string) (*Codec, error) {
	encoded, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("read configuration key: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil || len(key) != keySize {
		return nil, errors.New("configuration key must be Base64-encoded 32-byte data")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize configuration cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize configuration cipher: %w", err)
	}
	return &Codec{gcm: gcm}, nil
}

func GenerateKeyFile(path string) error {
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("generate configuration key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create configuration key: %w", err)
	}
	if _, err := fmt.Fprintln(file, base64.StdEncoding.EncodeToString(key)); err != nil {
		_ = file.Close()
		return fmt.Errorf("write configuration key: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close configuration key: %w", err)
	}
	return nil
}

func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, prefix)
}

func (c *Codec) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate encryption nonce: %w", err)
	}
	ciphertext := c.gcm.Seal(nil, nonce, plaintext, []byte(version))
	return fmt.Sprintf("%s%s.%s.%s)", prefix, version,
		base64.RawStdEncoding.EncodeToString(nonce),
		base64.RawStdEncoding.EncodeToString(ciphertext)), nil
}

func (c *Codec) Decrypt(value string) (string, error) {
	if !IsEncrypted(value) || !strings.HasSuffix(value, ")") {
		return "", errors.New("invalid encrypted value")
	}
	payload := strings.TrimSuffix(strings.TrimPrefix(value, prefix), ")")
	parts := strings.Split(payload, ".")
	if len(parts) != 3 || parts[0] != version {
		return "", errors.New("invalid encrypted value")
	}
	nonce, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil || len(nonce) != c.gcm.NonceSize() {
		return "", errors.New("invalid encrypted value")
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(ciphertext) < c.gcm.Overhead() {
		return "", errors.New("invalid encrypted value")
	}
	plaintext, err := c.gcm.Open(nil, nonce, ciphertext, []byte(version))
	if err != nil {
		return "", errors.New("invalid encrypted value or configuration key")
	}
	return string(plaintext), nil
}
