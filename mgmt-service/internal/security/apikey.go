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

func (k APIKeys) DefaultFor(domainID, userID string) (string, []byte) {
	mac := hmac.New(sha256.New, k.secret)
	_, _ = mac.Write([]byte("devbridge-api-key\x00"))
	_, _ = mac.Write([]byte(domainID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(userID))
	payload := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:apiKeyPayloadBytes])
	value := string(core.APIKeyScenarioDevBridge) + "_" + payload
	return value, apiKeyDigest(value)
}

func (APIKeys) New(scenario core.APIKeyScenario) (string, []byte) {
	random := make([]byte, apiKeyPayloadBytes)
	if _, err := rand.Read(random); err != nil {
		panic(err)
	}
	value := string(scenario) + "_" + base64.RawURLEncoding.EncodeToString(random)
	return value, apiKeyDigest(value)
}

func ValidAPIKeyScenario(scenario core.APIKeyScenario) bool {
	return scenario == core.APIKeyScenarioDevBridge || scenario == core.APIKeyScenarioDevBox
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

func splitAPIKey(value string) (core.APIKeyScenario, string, bool) {
	prefix, payload, found := strings.Cut(value, "_")
	scenario := core.APIKeyScenario(prefix)
	if !found || !ValidAPIKeyScenario(scenario) || len(payload) != apiKeyPayloadLength {
		return "", "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || len(decoded) != apiKeyPayloadBytes {
		return "", "", false
	}
	return scenario, payload, true
}

func apiKeyDigest(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	return digest[:]
}
