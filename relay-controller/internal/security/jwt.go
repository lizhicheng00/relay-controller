package security

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
	"unicode"

	"relay-controller/internal/core"
)

type JWTSigner struct {
	privateKey *rsa.PrivateKey
}

const (
	jwtIssuer   = "devbridge"
	jwtAudience = "relay-gateway"
	jwtKeyID    = "1"
	jwtLifetime = int64((24 * time.Hour) / time.Second)
)

func NewJWTSigner(configuredKey string) (*JWTSigner, error) {
	privateKey, err := parsePrivateKey(configuredKey)
	if err != nil {
		return nil, fmt.Errorf("jwt private key invalid: %w", err)
	}
	if privateKey.N.BitLen() < 2048 {
		return nil, fmt.Errorf("jwt RSA key must be at least 2048 bits")
	}
	return &JWTSigner{privateKey: privateKey}, nil
}

func (s *JWTSigner) Issue(tunnel core.Tunnel, scope string, now int64) (core.TunnelTokenResponse, error) {
	expiration := now + jwtLifetime
	jti, err := randomUUID()
	if err != nil {
		return core.TunnelTokenResponse{}, jwtGenerationError(err)
	}
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": jwtKeyID, "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iss": jwtIssuer, "aud": []string{jwtAudience}, "exp": expiration, "nbf": now, "jti": jti,
		"tunnelId": tunnel.TunnelID, "clusterId": tunnel.ClusterID, "scp": scope,
	})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return core.TunnelTokenResponse{}, jwtGenerationError(err)
	}
	return core.TunnelTokenResponse{
		TunnelID: tunnel.TunnelID, Scope: scope, Lifetime: jwtLifetime, Expiration: expiration,
		Token: unsigned + "." + base64.RawURLEncoding.EncodeToString(signature),
	}, nil
}

func jwtGenerationError(cause error) *core.AppError {
	return &core.AppError{Status: 500, Code: core.CodeJWTGenerateFailed, Message: "jwt generate failed", Cause: cause}
}

func parsePrivateKey(configured string) (*rsa.PrivateKey, error) {
	content := strings.TrimSpace(configured)
	if block, _ := pem.Decode([]byte(content)); block != nil {
		if block.Type != "PRIVATE KEY" {
			return nil, fmt.Errorf("private key must use PKCS#8 PRIVATE KEY format")
		}
		content = base64.StdEncoding.EncodeToString(block.Bytes)
	}
	content = strings.Map(func(character rune) rune {
		if unicode.IsSpace(character) {
			return -1
		}
		return character
	}, content)
	der, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("decode PKCS#8 key: %w", err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8 key: %w", err)
	}
	privateKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key must be RSA")
	}
	if err := privateKey.Validate(); err != nil {
		return nil, fmt.Errorf("validate RSA key: %w", err)
	}
	return privateKey, nil
}

func randomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}
