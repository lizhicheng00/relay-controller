package security

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"github.com/youmark/pkcs8"
)

func NewServerTLSConfig(certificateBase64, privateKeyBase64, password string) (*tls.Config, error) {
	if certificateBase64 == "" || privateKeyBase64 == "" {
		return nil, fmt.Errorf("server TLS certificate and private key are required")
	}
	certificatePEM, err := base64.StdEncoding.DecodeString(certificateBase64)
	if err != nil {
		return nil, fmt.Errorf("decode server TLS certificate: %w", err)
	}
	privateKeyPEM, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode server TLS private key: %w", err)
	}
	certificate, err := LoadKeyPair(certificatePEM, privateKeyPEM, password)
	if err != nil {
		return nil, fmt.Errorf("load server TLS certificate: %w", err)
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
		ClientAuth:   tls.NoClientCert,
	}, nil
}

func LoadKeyPair(certificatePEM, privateKeyPEM []byte, password string) (tls.Certificate, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil || block.Type != "ENCRYPTED PRIVATE KEY" {
		return tls.X509KeyPair(certificatePEM, privateKeyPEM)
	}
	if password == "" {
		return tls.Certificate{}, fmt.Errorf("private key password is required")
	}
	privateKey, err := pkcs8.ParsePKCS8PrivateKey(block.Bytes, []byte(password))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("decrypt private key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal private key: %w", err)
	}
	return tls.X509KeyPair(certificatePEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}
