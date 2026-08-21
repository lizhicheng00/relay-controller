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

var ErrInvalidAPIKey = errors.New("invalid API key")

func NewAPIKey(scope core.APIKeyScope) (string, []byte) {
	random := make([]byte, apiKeyPayloadBytes)
	if _, err := rand.Read(random); err != nil {
		panic(err)
	}
	value := string(scope) + "_" + base64.RawURLEncoding.EncodeToString(random)
	return value, apiKeyDigest(value)
}

func DeriveDefaultAPIKey(
	master [sha256.Size]byte,
	namespace string,
	scope core.APIKeyScope,
	keyID string,
) (string, []byte) {
	mac := hmac.New(sha256.New, master[:])
	_, _ = mac.Write([]byte(namespace))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(scope))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(keyID))
	payload := mac.Sum(nil)[:apiKeyPayloadBytes]
	value := string(scope) + "_" + base64.RawURLEncoding.EncodeToString(payload)
	return value, apiKeyDigest(value)
}

func ValidAPIKeyScope(scope core.APIKeyScope) bool {
	return scope == core.APIKeyScopeDevBridge || scope == core.APIKeyScopeDevBox
}

func MaskAPIKey(value string) string {
	prefix, payload, _ := strings.Cut(value, "_")
	return prefix + "_" + payload[:4] + "..." + payload[len(payload)-4:]
}

func ParseAPIKey(value string) (core.APIKeyScope, []byte, error) {
	scope, _, ok := splitAPIKey(value)
	if !ok {
		return "", nil, ErrInvalidAPIKey
	}
	return scope, apiKeyDigest(value), nil
}

func splitAPIKey(value string) (core.APIKeyScope, string, bool) {
	prefix, payload, found := strings.Cut(value, "_")
	scope := core.APIKeyScope(prefix)
	if !found || !ValidAPIKeyScope(scope) || len(payload) != apiKeyPayloadLength {
		return "", "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || len(decoded) != apiKeyPayloadBytes {
		return "", "", false
	}
	return scope, payload, true
}

func apiKeyDigest(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	return digest[:]
}
