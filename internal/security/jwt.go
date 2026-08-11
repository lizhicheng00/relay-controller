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

	"github.com/lizhicheng00/relay-controller/internal/core"
)

type JWTSigner struct {
	privateKey *rsa.PrivateKey
	issuer     string
	audience   string
	keyID      string
	lifetime   int64
}

func NewJWTSigner(configuredKey, issuer, audience, keyID string, lifetime time.Duration) (*JWTSigner, error) {
	privateKey, err := parsePrivateKey(configuredKey)
	if err != nil {
		return nil, fmt.Errorf("jwt private key invalid: %w", err)
	}
	if privateKey.N.BitLen() < 2048 {
		return nil, fmt.Errorf("jwt RSA key must be at least 2048 bits")
	}
	lifetimeSeconds := int64(lifetime / time.Second)
	if lifetimeSeconds <= 0 {
		return nil, fmt.Errorf("jwt token lifetime must be positive")
	}
	return &JWTSigner{
		privateKey: privateKey, issuer: issuer, audience: audience, keyID: keyID, lifetime: lifetimeSeconds,
	}, nil
}

func (s *JWTSigner) Issue(tunnel core.Tunnel, scope string, now int64) (core.TunnelTokenResponse, error) {
	expiration := now + s.lifetime
	jti, err := randomUUID()
	if err != nil {
		return core.TunnelTokenResponse{}, core.NewError(500, core.CodeJWTGenerateFailed, "jwt generate failed")
	}
	header, err := json.Marshal(map[string]any{"alg": "RS256", "kid": s.keyID, "typ": "JWT"})
	if err != nil {
		return core.TunnelTokenResponse{}, core.NewError(500, core.CodeJWTGenerateFailed, "jwt generate failed")
	}
	claims, err := json.Marshal(map[string]any{
		"iss": s.issuer, "aud": []string{s.audience}, "exp": expiration, "nbf": now, "jti": jti,
		"tunnelId": tunnel.TunnelID, "clusterId": tunnel.ClusterID, "scp": scope,
	})
	if err != nil {
		return core.TunnelTokenResponse{}, core.NewError(500, core.CodeJWTGenerateFailed, "jwt generate failed")
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return core.TunnelTokenResponse{}, core.NewError(500, core.CodeJWTGenerateFailed, "jwt generate failed")
	}
	return core.TunnelTokenResponse{
		TunnelID: tunnel.TunnelID, Scope: scope, Lifetime: s.lifetime, Expiration: expiration,
		Token: unsigned + "." + base64.RawURLEncoding.EncodeToString(signature),
	}, nil
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
