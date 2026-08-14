package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"mgmt-service/internal/core"
)

const (
	apiKeyPayloadBytes  = 24
	apiKeyPayloadLength = 32
)

var (
	ErrInvalidAPIKey = errors.New("invalid API key")
)

type APIKeys struct {
	secret []byte
}

func NewAPIKeys(secret string) APIKeys {
	return APIKeys{secret: []byte(secret)}
}

func (k APIKeys) DefaultFor(domainID, userID string, keyType core.APIKeyType) (string, []byte) {
	mac := hmac.New(sha256.New, k.secret)
	_, _ = mac.Write([]byte(string(keyType) + "-api-key\x00"))
	_, _ = mac.Write([]byte(domainID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(userID))
	payload := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:apiKeyPayloadBytes])
	value := string(keyType) + "_" + payload
	return value, apiKeyDigest(value)
}

func (APIKeys) New(keyType core.APIKeyType) (string, []byte) {
	random := make([]byte, apiKeyPayloadBytes)
	if _, err := rand.Read(random); err != nil {
		panic(err)
	}
	value := string(keyType) + "_" + base64.RawURLEncoding.EncodeToString(random)
	return value, apiKeyDigest(value)
}

func ValidAPIKeyType(keyType core.APIKeyType) bool {
	return keyType == core.APIKeyTypeDevBridge || keyType == core.APIKeyTypeDevBox
}

func MaskAPIKey(value string) string {
	prefix, payload, _ := strings.Cut(value, "_")
	return prefix + "_" + payload[:4] + "..." + payload[len(payload)-4:]
}

func DigestAPIKey(value string) ([]byte, error) {
	if _, _, ok := splitAPIKey(value); !ok {
		return nil, ErrInvalidAPIKey
	}
	return apiKeyDigest(value), nil
}

func splitAPIKey(value string) (core.APIKeyType, string, bool) {
	prefix, payload, found := strings.Cut(value, "_")
	keyType := core.APIKeyType(prefix)
	if !found || !ValidAPIKeyType(keyType) || len(payload) != apiKeyPayloadLength {
		return "", "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || len(decoded) != apiKeyPayloadBytes {
		return "", "", false
	}
	return keyType, payload, true
}

func apiKeyDigest(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	return digest[:]
}
