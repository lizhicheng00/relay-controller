package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"relay-controller/internal/security"
)

const (
	checkPath           = "/open-api-inner/v1/mgmt-service/api-keys/check"
	resolveIdentityPath = "/open-api-inner/v1/mgmt-service/identities/resolve"
)

var ErrUnauthorized = errors.New("authentication failed")

type Principal struct {
	AccountNamespace string `json:"accountNamespace"`
	Namespace        string `json:"namespace"`
	Scope            string `json:"scope"`
}

type Resolver interface {
	ResolveAPIKey(context.Context, string) (Principal, error)
	ResolveIdentity(context.Context, string, string) (Principal, error)
}

type Client struct {
	checkEndpoint    string
	identityEndpoint string
	httpClient       *http.Client
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
	checkEndpoint, err := url.JoinPath(baseURL, checkPath)
	if err != nil {
		return nil, fmt.Errorf("build API key check URL: %w", err)
	}
	identityEndpoint, err := url.JoinPath(baseURL, resolveIdentityPath)
	if err != nil {
		return nil, fmt.Errorf("build identity resolution URL: %w", err)
	}
	tlsConfig, err := newTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	return &Client{
		checkEndpoint:    checkEndpoint,
		identityEndpoint: identityEndpoint,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   3 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
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
	certificate, err := security.LoadKeyPair(certificatePEM, privateKeyPEM, cfg.ClientKeyPassword)
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

func (c *Client) ResolveAPIKey(ctx context.Context, apiKey string) (Principal, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.checkEndpoint, nil)
	if err != nil {
		return Principal{}, fmt.Errorf("create API key check request: %w", err)
	}
	request.Header.Set("X-API-Key", apiKey)
	return c.resolve(request)
}

func (c *Client) ResolveIdentity(ctx context.Context, domainID, userID string) (Principal, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.identityEndpoint, nil)
	if err != nil {
		return Principal{}, fmt.Errorf("create identity resolution request: %w", err)
	}
	request.Header.Set("X-Domain-Id", domainID)
	request.Header.Set("X-User-Id", userID)
	principal, err := c.resolve(request)
	if err != nil {
		return Principal{}, err
	}
	principal.Scope = "devbridge"
	return principal, nil
}

func (c *Client) resolve(request *http.Request) (Principal, error) {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return Principal{}, fmt.Errorf("resolve identity: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		if response.StatusCode == http.StatusBadRequest ||
			response.StatusCode == http.StatusUnauthorized ||
			response.StatusCode == http.StatusForbidden {
			return Principal{}, ErrUnauthorized
		}
		return Principal{}, fmt.Errorf("resolve identity: management service returned %s", response.Status)
	}
	var principal Principal
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&principal); err != nil {
		return Principal{}, fmt.Errorf("decode identity: %w", err)
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
