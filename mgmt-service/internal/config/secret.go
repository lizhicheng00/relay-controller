package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const encryptedConfigPrefix = "MGMT_SECRET_V1:"

func GenerateMasterKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate master key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func EncryptFile(inputPath, outputPath, encodedKey string) error {
	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}
	var values secretValues
	if err := json.Unmarshal(plaintext, &values); err != nil {
		return fmt.Errorf("decode configuration: %w", err)
	}
	ciphertext, err := encrypt(plaintext, encodedKey)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, ciphertext, 0o600); err != nil {
		return fmt.Errorf("write encrypted configuration: %w", err)
	}
	if err := os.Chmod(outputPath, 0o600); err != nil {
		return fmt.Errorf("restrict encrypted configuration permissions: %w", err)
	}
	return nil
}

func encrypt(plaintext []byte, encodedKey string) ([]byte, error) {
	gcm, err := newGCM(encodedKey)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, []byte(encryptedConfigPrefix))
	encoded := base64.RawStdEncoding.EncodeToString(sealed)
	return []byte(encryptedConfigPrefix + encoded), nil
}

func decrypt(value []byte, encodedKey string) ([]byte, error) {
	encoded := strings.TrimSpace(string(value))
	if !strings.HasPrefix(encoded, encryptedConfigPrefix) {
		return nil, errors.New("unsupported encrypted configuration format")
	}
	sealed, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(encoded, encryptedConfigPrefix))
	if err != nil {
		return nil, errors.New("invalid encrypted configuration")
	}
	gcm, err := newGCM(encodedKey)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("invalid encrypted configuration")
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(encryptedConfigPrefix))
	if err != nil {
		return nil, errors.New("invalid encrypted configuration or master key")
	}
	return plaintext, nil
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
