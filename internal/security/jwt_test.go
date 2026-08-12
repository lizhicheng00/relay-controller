package security

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lizhicheng00/relay-controller/internal/core"
)

func TestJWTSignerProducesExpectedClaimsAndSignature(t *testing.T) {
	privateKey := testPrivateKey(t, 2048)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewJWTSigner(base64.StdEncoding.EncodeToString(der))
	if err != nil {
		t.Fatal(err)
	}
	issued, err := signer.Issue(core.Tunnel{TunnelID: "aaaadysa", ClusterID: "cluster-a"}, "connect", 1000)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(issued.Token, ".")
	if len(parts) != 3 {
		t.Fatalf("token contains %d parts", len(parts))
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	if err := rsa.VerifyPKCS1v15(&privateKey.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
		t.Fatalf("verify token: %v", err)
	}
	claimsJSON, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		t.Fatal(err)
	}
	for _, claim := range []string{"iss", "aud", "exp", "nbf", "jti", "tunnelId", "clusterId", "scp"} {
		if _, exists := claims[claim]; !exists {
			t.Fatalf("claim %q is missing", claim)
		}
	}
	if len(claims) != 8 || claims["tunnelId"] != "aaaadysa" || claims["clusterId"] != "cluster-a" || claims["scp"] != "connect" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if issued.Lifetime != 86400 || issued.Expiration != 87400 {
		t.Fatalf("unexpected token timing: %#v", issued)
	}
}

func TestJWTSignerCreatesUniqueTokens(t *testing.T) {
	privateKey := testPrivateKey(t, 2048)
	der, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	signer, err := NewJWTSigner(base64.StdEncoding.EncodeToString(der))
	if err != nil {
		t.Fatal(err)
	}
	tunnel := core.Tunnel{TunnelID: "aaaadysa", ClusterID: "cluster-a"}
	first, _ := signer.Issue(tunnel, "host", 1000)
	second, _ := signer.Issue(tunnel, "host", 1000)
	if first.Token == second.Token {
		t.Fatal("tokens must have unique jti values")
	}
}

func TestJWTSignerRejectsWeakKey(t *testing.T) {
	privateKey := testPrivateKey(t, 1024)
	der, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	if _, err := NewJWTSigner(base64.StdEncoding.EncodeToString(der)); err == nil {
		t.Fatal("expected a 1024-bit key to be rejected")
	}
}

func testPrivateKey(t *testing.T, bits int) *rsa.PrivateKey {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	return privateKey
}
