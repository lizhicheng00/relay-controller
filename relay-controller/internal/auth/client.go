package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/youmark/pkcs8"
)

const checkPath = "/open-api-inner/v1/mgmt-service/api-keys/check"

var ErrUnauthorized = errors.New("API key authentication failed")

type Principal struct {
	AccountNamespace string `json:"accountNamespace"`
	Namespace        string `json:"namespace"`
	Scope            string `json:"scope"`
}

type Resolver interface {
	ResolveAPIKey(context.Context, string) (Principal, error)
}

type Client struct {
	endpoint   string
	httpClient *http.Client
}

type TLSConfig struct {
	ServerName        string
	ClientCertBase64  string
	ClientKeyBase64   string
	ClientKeyPassword string
	CACertBase64      string
}

func NewClient(baseURL string, cfg TLSConfig) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("management service URL must use HTTPS")
	}
	endpoint, err := url.JoinPath(baseURL, checkPath)
	if err != nil {
		return nil, fmt.Errorf("build management service URL: %w", err)
	}
	tlsConfig, err := newTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	return newClient(endpoint, &http.Client{
		Transport: transport,
		Timeout:   3 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}), nil
}

func newClient(endpoint string, httpClient *http.Client) *Client {
	return &Client{
		endpoint:   endpoint,
		httpClient: httpClient,
	}
}

func newTLSConfig(cfg TLSConfig) (*tls.Config, error) {
	if cfg.ClientCertBase64 == "" || cfg.ClientKeyBase64 == "" {
		return nil, fmt.Errorf("management service client certificate and key are required")
	}
	certificatePEM, err := base64.StdEncoding.DecodeString(cfg.ClientCertBase64)
	if err != nil {
		return nil, fmt.Errorf("decode management service client certificate: %w", err)
	}
	privateKeyPEM, err := base64.StdEncoding.DecodeString(cfg.ClientKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode management service client key: %w", err)
	}
	certificate, err := loadKeyPair(certificatePEM, privateKeyPEM, cfg.ClientKeyPassword)
	if err != nil {
		return nil, fmt.Errorf("load management service client certificate: %w", err)
	}

	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system certificate pool: %w", err)
	}
	if cfg.CACertBase64 != "" {
		caPEM, err := base64.StdEncoding.DecodeString(cfg.CACertBase64)
		if err != nil {
			return nil, fmt.Errorf("decode management service CA certificate: %w", err)
		}
		if !roots.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("management service CA certificate is invalid")
		}
	} else if len(certificate.Certificate) > 1 {
		issuer, err := x509.ParseCertificate(certificate.Certificate[len(certificate.Certificate)-1])
		if err != nil {
			return nil, fmt.Errorf("parse client certificate issuer: %w", err)
		}
		roots.AddCert(issuer)
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		ServerName:   cfg.ServerName,
		Certificates: []tls.Certificate{certificate},
		RootCAs:      roots,
	}, nil
}

func loadKeyPair(certificatePEM, privateKeyPEM []byte, password string) (tls.Certificate, error) {
	certificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err == nil || password == "" {
		return certificate, err
	}
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil || block.Type != "ENCRYPTED PRIVATE KEY" {
		return tls.Certificate{}, err
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

func (c *Client) ResolveAPIKey(ctx context.Context, apiKey string) (Principal, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, nil)
	if err != nil {
		return Principal{}, fmt.Errorf("create API key check request: %w", err)
	}
	request.Header.Set("X-API-Key", apiKey)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return Principal{}, fmt.Errorf("check API key: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return Principal{}, ErrUnauthorized
	}
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return Principal{}, fmt.Errorf("check API key: management service returned %s", response.Status)
	}
	var principal Principal
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&principal); err != nil {
		return Principal{}, fmt.Errorf("decode API key identity: %w", err)
	}
	return principal, nil
}

type contextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(contextKey{}).(Principal)
	return principal, ok
}
