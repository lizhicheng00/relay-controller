package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"math/big"
	"strings"
)

const apiKeyLength = 32

var ErrInvalidAPIKey = errors.New("invalid API key")

type APIKeys struct {
	secret []byte
}

func NewAPIKeys(secret string) APIKeys {
	return APIKeys{secret: []byte(secret)}
}

func (k APIKeys) For(domainID, userID string) (string, []byte) {
	mac := hmac.New(sha256.New, k.secret)
	_, _ = mac.Write([]byte("devbridge-api-key\x00"))
	_, _ = mac.Write([]byte(domainID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(userID))
	encoded := new(big.Int).SetBytes(mac.Sum(nil)).Text(36)
	if len(encoded) < apiKeyLength {
		encoded = strings.Repeat("0", apiKeyLength-len(encoded)) + encoded
	}
	value := encoded[len(encoded)-apiKeyLength:]
	return value, apiKeyDigest(value)
}

func DigestAPIKey(value string) ([]byte, error) {
	if len(value) != apiKeyLength {
		return nil, ErrInvalidAPIKey
	}
	for _, char := range value {
		if char < 'a' || char > 'z' {
			if char < '0' || char > '9' {
				return nil, ErrInvalidAPIKey
			}
		}
	}
	return apiKeyDigest(value), nil
}

func apiKeyDigest(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	return digest[:]
}
