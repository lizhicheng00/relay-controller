package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

const (
	prefix         = "ENC("
	keySize        = 32
	defaultKeyFile = "/opt/cloud/dog/beta"
)

var state struct {
	sync.Mutex
	keyFile string
	codec   *codec
}

type codec struct {
	gcm cipher.AEAD
}

func Init() error {
	state.Lock()
	defer state.Unlock()
	state.keyFile = valueOrDefault("RELAY_CONFIG_KEY_FILE", defaultKeyFile)
	state.codec = nil
	return nil
}

func GetEncryptedEnv(name string) (string, error) {
	value := os.Getenv(name)
	if !strings.HasPrefix(value, prefix) {
		return value, nil
	}
	c, err := configuredCodec()
	if err != nil {
		return "", err
	}
	return c.decrypt(value)
}

func configuredCodec() (*codec, error) {
	state.Lock()
	defer state.Unlock()
	if state.codec != nil {
		return state.codec, nil
	}
	keyFile := state.keyFile
	if keyFile == "" {
		keyFile = valueOrDefault("RELAY_CONFIG_KEY_FILE", defaultKeyFile)
	}
	c, err := load(keyFile)
	if err != nil {
		return nil, err
	}
	state.codec = c
	return c, nil
}

func load(keyFile string) (*codec, error) {
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
	return &codec{gcm: gcm}, nil
}

func (c *codec) decrypt(value string) (string, error) {
	if !strings.HasSuffix(value, ")") {
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

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
