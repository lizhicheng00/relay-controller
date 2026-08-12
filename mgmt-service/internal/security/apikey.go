package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

const (
	alphabet  = "abcdefghijklmnopqrstuvwxyz0123456789"
	keyLength = 32
	// 252 is the largest multiple of 36 below 256. Rejecting larger bytes avoids modulo bias.
	maxByte = 252
)

var ErrInvalid = errors.New("invalid API key")

type APIKeyCodec struct {
	pepper []byte
}

func NewAPIKeyCodec(pepper string) APIKeyCodec {
	return APIKeyCodec{pepper: []byte(pepper)}
}

func (c APIKeyCodec) Generate() (value string, mask string, hash []byte, err error) {
	key := make([]byte, keyLength)
	buffer := make([]byte, keyLength)
	for index := 0; index < len(key); {
		if _, err = rand.Read(buffer); err != nil {
			return "", "", nil, err
		}
		for _, value := range buffer {
			if value >= maxByte {
				continue
			}
			key[index] = alphabet[int(value)%len(alphabet)]
			index++
			if index == len(key) {
				break
			}
		}
	}
	value = string(key)
	return value, value[:2] + "..." + value[len(value)-4:], c.digest(value), nil
}

func (c APIKeyCodec) Digest(value string) ([]byte, error) {
	if len(value) != keyLength {
		return nil, ErrInvalid
	}
	for _, char := range value {
		if char < 'a' || char > 'z' {
			if char < '0' || char > '9' {
				return nil, ErrInvalid
			}
		}
	}
	return c.digest(value), nil
}

func (c APIKeyCodec) digest(value string) []byte {
	mac := hmac.New(sha256.New, c.pepper)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
