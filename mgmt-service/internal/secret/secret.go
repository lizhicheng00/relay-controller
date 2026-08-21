package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	prefix  = "ENC("
	keySize = 32
)

type Codec struct {
	gcm cipher.AEAD
	key [keySize]byte
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
	codec := &Codec{gcm: gcm}
	copy(codec.key[:], key)
	return codec, nil
}

func (c *Codec) DeriveKey(purpose string) [keySize]byte {
	mac := hmac.New(sha256.New, c.key[:])
	_, _ = mac.Write([]byte(purpose))
	var derived [keySize]byte
	copy(derived[:], mac.Sum(nil))
	return derived
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
	sealed := c.gcm.Seal(nonce, nonce, plaintext, nil)
	return prefix + base64.RawStdEncoding.EncodeToString(sealed) + ")", nil
}

func (c *Codec) Decrypt(value string) (string, error) {
	if !IsEncrypted(value) || !strings.HasSuffix(value, ")") {
		return "", errors.New("invalid encrypted value")
	}
	payload := strings.TrimSuffix(strings.TrimPrefix(value, prefix), ")")
	sealed, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil || len(sealed) < c.gcm.NonceSize() {
		return "", errors.New("invalid encrypted value")
	}
	nonce, ciphertext := sealed[:c.gcm.NonceSize()], sealed[c.gcm.NonceSize():]
	plaintext, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("invalid encrypted value or configuration key")
	}
	return string(plaintext), nil
}
