package security

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
)

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func NewID(prefix string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random identifier: %w", err)
	}
	return prefix + strings.ToLower(encoding.EncodeToString(bytes)), nil
}
