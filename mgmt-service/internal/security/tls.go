package security

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"

	"mgmt-service/internal/config"
	"software.sslmate.com/src/go-pkcs12"
)

func TLSConfig(cfg config.TLS) (*tls.Config, error) {
	keyStore, err := base64.StdEncoding.DecodeString(cfg.KeyStoreBase64)
	if err != nil {
		return nil, fmt.Errorf("decode server key store: %w", err)
	}
	privateKey, certificate, chain, err := pkcs12.DecodeChain(keyStore, cfg.KeyStorePassword)
	if err != nil {
		return nil, fmt.Errorf("decode server key store: %w", err)
	}
	certificates := make([][]byte, 0, len(chain)+1)
	certificates = append(certificates, certificate.Raw)
	for _, issuer := range chain {
		certificates = append(certificates, issuer.Raw)
	}

	trustStore, err := base64.StdEncoding.DecodeString(cfg.TrustStoreBase64)
	if err != nil {
		return nil, fmt.Errorf("decode trust store: %w", err)
	}
	trustedCertificates, err := pkcs12.DecodeTrustStore(trustStore, cfg.TrustStorePassword)
	if err != nil {
		return nil, fmt.Errorf("decode trust store: %w", err)
	}
	clientCAs := x509.NewCertPool()
	for _, trusted := range trustedCertificates {
		clientCAs.AddCert(trusted)
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		Certificates: []tls.Certificate{{
			Certificate: certificates,
			PrivateKey:  privateKey,
			Leaf:        certificate,
		}},
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCAs,
	}, nil
}
