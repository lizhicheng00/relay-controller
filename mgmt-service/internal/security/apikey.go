package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"
	"strings"
)

const apiKeyLength = 32

const base36Alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

var ErrInvalidAPIKey = errors.New("invalid API key")

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
	encoded := new(big.Int).SetBytes(mac.Sum(nil)).Text(36)
	if len(encoded) < apiKeyLength {
		encoded = strings.Repeat("0", apiKeyLength-len(encoded)) + encoded
	}
	value := encoded[len(encoded)-apiKeyLength:]
	return value, apiKeyDigest(value)
}

func (APIKeys) New() (string, []byte, error) {
	value := make([]byte, apiKeyLength)
	random := make([]byte, apiKeyLength*2)
	for index := 0; index < len(value); {
		if _, err := rand.Read(random); err != nil {
			return "", nil, err
		}
		for _, number := range random {
			if number >= 252 {
				continue
			}
			value[index] = base36Alphabet[int(number)%len(base36Alphabet)]
			index++
			if index == len(value) {
				break
			}
		}
	}
	return string(value), apiKeyDigest(string(value)), nil
}

func MaskAPIKey(value string) string {
	if len(value) < 8 {
		return "********"
	}
	return value[:4] + "..." + value[len(value)-4:]
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
