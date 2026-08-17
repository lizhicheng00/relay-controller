package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const encryptedValuePrefix = "ENC("

func GenerateMasterKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate master key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func EncryptValue(plaintext, encodedKey string) (string, error) {
	gcm, err := newGCM(encodedKey)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedValuePrefix + base64.RawStdEncoding.EncodeToString(sealed) + ")", nil
}

func decryptIfEncrypted(value, encodedKey string) (string, error) {
	if !strings.HasPrefix(value, encryptedValuePrefix) {
		return value, nil
	}
	if !strings.HasSuffix(value, ")") {
		return "", errors.New("unsupported encrypted value format")
	}
	payload := strings.TrimSuffix(strings.TrimPrefix(value, encryptedValuePrefix), ")")
	sealed, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		return "", errors.New("invalid encrypted value")
	}
	gcm, err := newGCM(encodedKey)
	if err != nil {
		return "", err
	}
	if len(sealed) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted value")
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("invalid encrypted value or master key")
	}
	return string(plaintext), nil
}

func newGCM(encodedKey string) (cipher.AEAD, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil || len(key) != 32 {
		return nil, errors.New("MGMT_CONFIG_MASTER_KEY must be Base64-encoded 32-byte data")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize configuration cipher: %w", err)
	}
	return cipher.NewGCM(block)
}
