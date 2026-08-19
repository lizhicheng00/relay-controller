package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
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

func NewClient(baseURL string) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("management service URL is invalid")
	}
	endpoint, err := url.JoinPath(baseURL, checkPath)
	if err != nil {
		return nil, fmt.Errorf("build management service URL: %w", err)
	}
	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
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
