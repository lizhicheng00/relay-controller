package core

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"
	_ "time/tzdata"
)

const Max40Bit = uint64(1<<40) - 1

var shanghai = mustLocation("Asia/Shanghai")

func ValidIdentifier(value string) bool {
	if len(value) < 1 || len(value) > 128 || !asciiLetterOrDigit(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !asciiLetterOrDigit(character) && character != '.' && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func ValidTunnelID(value string) bool {
	if len(value) != 8 {
		return false
	}
	for index := range value {
		if (value[index] < 'a' || value[index] > 'z') && (value[index] < '2' || value[index] > '7') {
			return false
		}
	}
	return true
}

func NormalizeTunnelType(value *string, defaultBridge bool) (string, error) {
	if value == nil {
		if defaultBridge {
			return "bridge", nil
		}
		return "", nil
	}
	normalized := strings.ToLower(strings.TrimSpace(*value))
	if normalized == "" && defaultBridge {
		return "bridge", nil
	}
	if normalized != "bridge" && normalized != "env" {
		return "", InvalidField("type", "must be bridge or env")
	}
	return normalized, nil
}

func NormalizeProtocol(value *string, required bool) (string, error) {
	if value == nil {
		if required {
			return "", InvalidField("protocol", "is required")
		}
		return "", nil
	}
	normalized := strings.ToLower(strings.TrimSpace(*value))
	if normalized != "http" && normalized != "https" && normalized != "auto" {
		return "", InvalidField("protocol", "must be http, https, or auto")
	}
	return normalized, nil
}

func ResolveExpiration(hours *int, fallback int, now int64) (int, int64, error) {
	resolved := fallback
	if hours != nil {
		resolved = *hours
	}
	if resolved < 1 || resolved > 720 {
		return 0, 0, InvalidField("expiration", "must be between 1 and 720")
	}
	expiration := now + int64(resolved)*3600
	if expiration > math.MaxUint32 {
		return 0, 0, InvalidField("expiration", "is too large")
	}
	return resolved, expiration, nil
}

func BillingPeriodRange(timestamp int64) (int64, int64) {
	local := time.Unix(timestamp, 0).In(shanghai)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, shanghai)
	return start.Unix(), start.AddDate(0, 1, 0).Unix()
}

func NewTunnelCode() (uint64, string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[3:]); err != nil {
		return 0, "", fmt.Errorf("generate tunnel code: %w", err)
	}
	code := binary.BigEndian.Uint64(bytes[:]) & Max40Bit
	if code == 0 {
		code = 1
	}
	return code, Encode40Bit(code), nil
}

func Encode40Bit(value uint64) string {
	bytes := []byte{byte(value >> 32), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes))
}

func AddBytes(first, second uint64) (uint64, error) {
	if math.MaxUint64-first < second {
		return 0, fmt.Errorf("byte counter overflow")
	}
	return first + second, nil
}

func asciiLetterOrDigit(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

func mustLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return location
}
