package security

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/youmark/pkcs8"
)

func TestServerTLSConfigLoadsEncryptedPrivateKey(t *testing.T) {
	privateKey := testPrivateKey(t, 2048)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "relay.example.com"},
		DNSNames:     []string{"relay.example.com"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certificateDER, err := x509.CreateCertificate(
		rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	encryptedKey, err := pkcs8.MarshalPrivateKey(privateKey, []byte("password"), nil)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: encryptedKey})

	configuration, err := NewServerTLSConfig(
		base64.StdEncoding.EncodeToString(certificatePEM),
		base64.StdEncoding.EncodeToString(privateKeyPEM),
		"password",
	)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.MinVersion != tls.VersionTLS12 ||
		configuration.ClientAuth != tls.NoClientCert || len(configuration.Certificates) != 1 {
		t.Fatalf("TLS configuration = %#v", configuration)
	}
}

func TestServerTLSConfigRequiresCertificateAndKey(t *testing.T) {
	if _, err := NewServerTLSConfig("", "", ""); err == nil {
		t.Fatal("NewServerTLSConfig() accepted missing certificate material")
	}
}

func TestLoadKeyPairSupportsLegacyEncryptedKeyAndCertificateChain(t *testing.T) {
	privateKey := testPrivateKey(t, 2048)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "relay-controller"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	certificateDER, err := x509.CreateCertificate(
		rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificateBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	certificateChain := append(append(append([]byte{}, certificateBlock...), certificateBlock...), certificateBlock...)
	encryptedBlock, err := x509.EncryptPEMBlock(
		rand.Reader, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(privateKey), []byte("password"), x509.PEMCipherAES256)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := append(append([]byte{}, certificateBlock...), pem.EncodeToMemory(encryptedBlock)...)

	certificate, err := LoadKeyPair(certificateChain, keyPEM, "password")
	if err != nil {
		t.Fatal(err)
	}
	if len(certificate.Certificate) != 3 {
		t.Fatalf("certificate chain length = %d, want 3", len(certificate.Certificate))
	}
}
