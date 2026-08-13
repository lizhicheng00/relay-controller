package security

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func NewID(prefix string) string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return prefix + strings.ToLower(encoding.EncodeToString(value))
}
