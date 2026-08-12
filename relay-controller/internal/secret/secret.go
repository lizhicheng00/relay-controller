package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	KeyEnvironment = "RELAY_CONFIG_DECRYPTION_KEY"
	prefix         = "ENC("
	version        = "v1"
)

// Resolve decrypts encrypted configuration and leaves local plaintext unchanged.
func Resolve(name, value, encodedKey string) (string, error) {
	if !strings.HasPrefix(value, prefix) {
		return value, nil
	}
	if !strings.HasSuffix(value, ")") {
		return "", fmt.Errorf("invalid encrypted value")
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(value, prefix), ")"), ".")
	if len(parts) != 3 || parts[0] != version {
		return "", fmt.Errorf("unsupported encrypted value format")
	}
	gcm, err := newGCM(encodedKey)
	if err != nil {
		return "", err
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(nonce) != gcm.NonceSize() {
		return "", fmt.Errorf("invalid encrypted value nonce")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid encrypted value payload")
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(name))
	if err != nil {
		return "", fmt.Errorf("encrypted value authentication failed")
	}
	return string(plaintext), nil
}

// Encrypt creates an authenticated value bound to its configuration name.
func Encrypt(name, value, encodedKey string) (string, error) {
	gcm, err := newGCM(encodedKey)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate encrypted value nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(value), []byte(name))
	return prefix + strings.Join([]string{
		version,
		base64.RawURLEncoding.EncodeToString(nonce),
		base64.RawURLEncoding.EncodeToString(ciphertext),
	}, ".") + ")", nil
}

func newGCM(encodedKey string) (cipher.AEAD, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s must be a Base64-encoded 32-byte key", KeyEnvironment)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create configuration cipher: %w", err)
	}
	return cipher.NewGCM(block)
}
