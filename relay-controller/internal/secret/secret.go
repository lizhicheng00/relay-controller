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
	prefix        = "ENC("
	version       = "v1"
	componentSize = 32
	encodedCat    = "2RVGMfRmj8kpmd/VhS0RNimLyfr6bPRq0NUreIY0EY0="
)

type Codec struct {
	gcm cipher.AEAD
}

func Load(dogFile, omega string) (*Codec, error) {
	encodedDog, err := os.ReadFile(dogFile)
	if err != nil {
		return nil, fmt.Errorf("read dog component: %w", err)
	}
	dog, err := decodeComponent("dog", string(encodedDog))
	if err != nil {
		return nil, err
	}
	cat, err := decodeComponent("cat", encodedCat)
	if err != nil {
		return nil, err
	}
	pig, err := decodeComponent("pig", os.Getenv(omega))
	if err != nil {
		return nil, err
	}
	key := make([]byte, componentSize)
	// XOR reconstructs a 3-of-3 key without exposing it in any single component.
	for i := range key {
		key[i] = dog[i] ^ cat[i] ^ pig[i]
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

func decodeComponent(name, encoded string) ([]byte, error) {
	component, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(component) != componentSize {
		return nil, fmt.Errorf("%s component must be Base64-encoded 32-byte data", name)
	}
	return component, nil
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
