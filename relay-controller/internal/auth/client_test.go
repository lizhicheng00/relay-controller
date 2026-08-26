package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/youmark/pkcs8"
)

func TestClientResolvesAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != checkPath || request.Header.Get("X-API-Key") != "devbridge_test" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"accountNamespace":"ns-account","namespace":"ns-user","scope":"devbridge"}`))
	}))
	defer server.Close()
	client := &Client{checkEndpoint: server.URL + checkPath, httpClient: server.Client()}
	principal, err := client.ResolveAPIKey(context.Background(), "devbridge_test")
	if err != nil || principal.Namespace != "ns-user" || principal.AccountNamespace != "ns-account" || principal.Scope != "devbridge" {
		t.Fatalf("ResolveAPIKey() = %#v, %v", principal, err)
	}
}

func TestClientMapsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	client := &Client{checkEndpoint: server.URL + checkPath, httpClient: server.Client()}
	if _, err := client.ResolveAPIKey(context.Background(), "invalid"); err != ErrUnauthorized {
		t.Fatalf("ResolveAPIKey() error = %v", err)
	}
}

func TestClientResolvesTrustedIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != resolveIdentityPath ||
			request.Header.Get("X-Domain-Id") != "domain-1" || request.Header.Get("X-User-Id") != "user-1" {
			t.Fatalf("unexpected request: %s %s %#v", request.Method, request.URL.Path, request.Header)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"accountNamespace":"ns-account","namespace":"ns-user"}`))
	}))
	defer server.Close()
	client := &Client{identityEndpoint: server.URL + resolveIdentityPath, httpClient: server.Client()}

	principal, err := client.ResolveIdentity(context.Background(), "domain-1", "user-1")
	if err != nil || principal.Namespace != "ns-user" || principal.AccountNamespace != "ns-account" ||
		principal.Scope != "devbridge" {
		t.Fatalf("ResolveIdentity() = %#v, %v", principal, err)
	}
}

func TestClientRejectsInvalidURL(t *testing.T) {
	if _, err := NewClient("http://localhost:8444", TLSConfig{}); err == nil {
		t.Fatal("NewClient() accepted an HTTP URL")
	}
}

func TestTLSConfigLoadsEncryptedClientKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "relay-controller"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	encryptedKey, err := pkcs8.MarshalPrivateKey(privateKey, []byte("password"), nil)
	if err != nil {
		t.Fatal(err)
	}
	certificateBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	certificatePEM := append(append(append([]byte{}, certificateBlock...), certificateBlock...), certificateBlock...)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: encryptedKey})
	tlsConfig, err := newTLSConfig(TLSConfig{
		ServerName:        "huaweicloud.com",
		ClientCertBase64:  base64.StdEncoding.EncodeToString(certificatePEM),
		ClientKeyBase64:   base64.StdEncoding.EncodeToString(privateKeyPEM),
		ClientKeyPassword: "password",
		CACertBase64:      base64.StdEncoding.EncodeToString(certificatePEM),
	})
	if err != nil {
		t.Fatal(err)
	}
	if tlsConfig.ServerName != "huaweicloud.com" || len(tlsConfig.Certificates) != 1 ||
		len(tlsConfig.Certificates[0].Certificate) != 3 || tlsConfig.RootCAs == nil {
		t.Fatal("client certificate or CA pool was not loaded")
	}
	withoutCustomCA, err := newTLSConfig(TLSConfig{
		ClientCertBase64:  base64.StdEncoding.EncodeToString(certificatePEM),
		ClientKeyBase64:   base64.StdEncoding.EncodeToString(privateKeyPEM),
		ClientKeyPassword: "password",
	})
	if err != nil {
		t.Fatalf("newTLSConfig() without custom CA error = %v", err)
	}
	parsedCertificate, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parsedCertificate.Verify(x509.VerifyOptions{
		Roots:     withoutCustomCA.RootCAs,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		t.Fatalf("client certificate chain issuer was not trusted: %v", err)
	}
}
