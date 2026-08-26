package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"relay-controller/internal/auth"
	"relay-controller/internal/core"
)

var testPrincipal = auth.Principal{
	Namespace: "ns-sub-user-001", AccountNamespace: "ns-user-001", Scope: "devbridge",
}

const testTrustedIdentityToken = "trusted-identity-token"

func TestCreateTunnelReturnsDirectResponse(t *testing.T) {
	api := stubAPI{createTunnel: func(_ context.Context, namespace, accountNamespace string, request core.CreateTunnelRequest) (core.TunnelResponse, error) {
		if namespace != "ns-sub-user-001" || accountNamespace != "ns-user-001" || request.ClusterID != "cluster-a" {
			t.Fatalf("unexpected request context: %q %q %#v", namespace, accountNamespace, request)
		}
		return core.TunnelResponse{TunnelID: "aaaadysa", ClusterID: "cluster-a", ExpirationHours: 72}, nil
	}}
	response := serve(t, api, http.MethodPost, apiBase+"/tunnels", `{"name":"dev","clusterId":"cluster-a"}`, true)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, `"tunnelId":"aaaadysa"`) || strings.Contains(body, `"data"`) || strings.Contains(body, `"result"`) {
		t.Fatalf("unexpected response: %s", body)
	}
}

func TestMissingAPIKeyReturnsStructured401(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, apiBase+"/tunnels", strings.NewReader(`{"name":"dev"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	New(stubAPI{}, stubResolver{principal: testPrincipal}, "", testLogger(), nil).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":"40100"`) || !strings.Contains(response.Body.String(), `"target":"X-API-Key"`) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestAuthenticationCheckReturnsNoContent(t *testing.T) {
	response := serve(t, stubAPI{}, http.MethodGet, apiBase+"/auth/check", "", true)
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected response: %d, headers = %#v, body = %s", response.Code, response.Header(), response.Body.String())
	}
}

func TestAPIKeyAuthenticationFailures(t *testing.T) {
	tests := []struct {
		name     string
		resolver stubResolver
		status   int
		code     string
	}{
		{name: "invalid", resolver: stubResolver{err: auth.ErrUnauthorized}, status: http.StatusUnauthorized, code: core.CodeUnauthorized},
		{name: "scope", resolver: stubResolver{principal: auth.Principal{Scope: "devbox"}}, status: http.StatusForbidden, code: core.CodeForbidden},
		{name: "unavailable", resolver: stubResolver{err: errors.New("mgmt unavailable")}, status: http.StatusServiceUnavailable, code: core.CodeServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, apiBase+"/tunnels", nil)
			request.Header.Set("X-API-Key", "devbridge_test")
			response := httptest.NewRecorder()
			New(stubAPI{}, test.resolver, "", testLogger(), nil).ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestIssueTokenPreventsCaching(t *testing.T) {
	api := stubAPI{issueToken: func(context.Context, string, string, string) (core.TunnelTokenResponse, error) {
		return core.TunnelTokenResponse{TunnelID: "aaaadysa", Scope: "host", Lifetime: 3600, Expiration: 4600, Token: "token"}, nil
	}}
	response := serve(t, api, http.MethodPost, apiBase+"/tunnels/aaaadysa/token?scope=host", "", true)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected token response: %d %#v", response.Code, response.Header())
	}
}

func TestMalformedBodyReturnsRequestBodyTarget(t *testing.T) {
	response := serve(t, stubAPI{}, http.MethodPost, apiBase+"/tunnels", `{`, true)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"target":"requestBody"`) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestInvalidJSONMediaTypeIsRejected(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, apiBase+"/tunnels", strings.NewReader(`{"name":"dev"}`))
	request.Header.Set("Content-Type", "application/json-malformed")
	request.Header.Set("X-API-Key", "devbridge_test")
	response := httptest.NewRecorder()
	New(stubAPI{}, stubResolver{principal: testPrincipal}, "", testLogger(), nil).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"target":"requestBody"`) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestInternalErrorDoesNotLeakCause(t *testing.T) {
	api := stubAPI{getTunnel: func(context.Context, string, string) (core.TunnelResponse, error) {
		return core.TunnelResponse{}, errors.New("database password leaked")
	}}
	response := serve(t, api, http.MethodGet, apiBase+"/tunnels/aaaadysa", "", true)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "password") || !strings.Contains(response.Body.String(), `"code":"50000"`) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestPortCollectionDoesNotSupportDelete(t *testing.T) {
	response := serve(t, stubAPI{}, http.MethodDelete, apiBase+"/tunnels/aaaadysa/ports", "", true)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.Code)
	}
}

func TestTrustedIdentityUsesResolvedPrincipal(t *testing.T) {
	resolved := false
	resolver := stubResolver{resolveIdentity: func(_ context.Context, domainID, userID string) (auth.Principal, error) {
		resolved = true
		if domainID != "domain-1" || userID != "user-1" {
			t.Fatalf("identity = %q %q", domainID, userID)
		}
		return testPrincipal, nil
	}}
	api := stubAPI{createTunnel: func(_ context.Context, namespace, accountNamespace string, _ core.CreateTunnelRequest) (core.TunnelResponse, error) {
		if namespace != testPrincipal.Namespace || accountNamespace != testPrincipal.AccountNamespace {
			t.Fatalf("principal = %q %q", namespace, accountNamespace)
		}
		return core.TunnelResponse{}, nil
	}}
	request := httptest.NewRequest(http.MethodPost, apiBase+"/tunnels", strings.NewReader(`{"name":"dev","clusterId":"cluster-a"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(trustedIdentityTokenHeader, testTrustedIdentityToken)
	request.Header.Set(domainIDHeader, "domain-1")
	request.Header.Set(userIDHeader, "user-1")
	response := httptest.NewRecorder()

	New(api, resolver, testTrustedIdentityToken, testLogger(), nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK || !resolved {
		t.Fatalf("status = %d, resolved = %t, body = %s", response.Code, resolved, response.Body)
	}
}

func TestTrustedIdentityRejectsInvalidOrAmbiguousCredential(t *testing.T) {
	tests := []struct {
		configuredToken string
		configure       func(*http.Request)
	}{
		{configuredToken: testTrustedIdentityToken, configure: func(request *http.Request) {
			request.Header.Set(trustedIdentityTokenHeader, "invalid")
		}},
		{configure: func(request *http.Request) {
			request.Header.Set(trustedIdentityTokenHeader, testTrustedIdentityToken)
		}},
		{configuredToken: testTrustedIdentityToken, configure: func(request *http.Request) {
			request.Header.Set(trustedIdentityTokenHeader, testTrustedIdentityToken)
			request.Header.Set(apiKeyHeader, "devbridge_test")
		}},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, apiBase+"/tunnels", nil)
		request.Header.Set(domainIDHeader, "domain-1")
		request.Header.Set(userIDHeader, "user-1")
		test.configure(request)
		response := httptest.NewRecorder()
		New(stubAPI{}, stubResolver{principal: testPrincipal}, test.configuredToken,
			testLogger(), nil).ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body)
		}
	}
}

func TestRateLimiterUsesFixedNamespaceWindow(t *testing.T) {
	limiter := NewRateLimiter(2)
	now := time.Unix(1000, 0)
	limiter.now = func() time.Time { return now }
	if !limiter.Allow("namespace:ns-user-001") || !limiter.Allow("namespace:ns-user-001") || limiter.Allow("namespace:ns-user-001") {
		t.Fatal("expected third request to be limited")
	}
	now = now.Add(time.Minute)
	if !limiter.Allow("namespace:ns-user-001") {
		t.Fatal("expected a new window to allow requests")
	}
}

func serve(t *testing.T, api API, method, target, body string, authenticated bool) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		request.Header.Set("X-API-Key", "devbridge_test")
		request.Header.Set("X-Namespace", "forged-namespace")
		request.Header.Set("X-Account-Namespace", "forged-account")
	}
	response := httptest.NewRecorder()
	New(api, stubResolver{principal: testPrincipal}, "", testLogger(), nil).ServeHTTP(response, request)
	return response
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type stubAPI struct {
	createTunnel func(context.Context, string, string, core.CreateTunnelRequest) (core.TunnelResponse, error)
	getTunnel    func(context.Context, string, string) (core.TunnelResponse, error)
	issueToken   func(context.Context, string, string, string) (core.TunnelTokenResponse, error)
}

type stubResolver struct {
	principal       auth.Principal
	err             error
	resolveIdentity func(context.Context, string, string) (auth.Principal, error)
}

func (s stubResolver) ResolveAPIKey(context.Context, string) (auth.Principal, error) {
	return s.principal, s.err
}

func (s stubResolver) ResolveIdentity(ctx context.Context, domainID, userID string) (auth.Principal, error) {
	if s.resolveIdentity != nil {
		return s.resolveIdentity(ctx, domainID, userID)
	}
	return s.principal, s.err
}

func (s stubAPI) CreateTunnel(ctx context.Context, namespace, account string, request core.CreateTunnelRequest) (core.TunnelResponse, error) {
	if s.createTunnel != nil {
		return s.createTunnel(ctx, namespace, account, request)
	}
	return core.TunnelResponse{}, nil
}
func (stubAPI) ListTunnels(context.Context, string, string) ([]core.TunnelListItem, error) {
	return []core.TunnelListItem{}, nil
}
func (s stubAPI) GetTunnel(ctx context.Context, namespace, tunnelID string) (core.TunnelResponse, error) {
	if s.getTunnel != nil {
		return s.getTunnel(ctx, namespace, tunnelID)
	}
	return core.TunnelResponse{}, nil
}
func (stubAPI) UpdateTunnel(context.Context, string, string, core.UpdateTunnelRequest) (bool, error) {
	return true, nil
}
func (stubAPI) DeleteTunnel(context.Context, string, string) (bool, error) { return true, nil }
func (stubAPI) DeleteTunnels(context.Context, string) (bool, error)        { return true, nil }
func (s stubAPI) IssueTunnelToken(ctx context.Context, namespace, tunnelID, scope string) (core.TunnelTokenResponse, error) {
	if s.issueToken != nil {
		return s.issueToken(ctx, namespace, tunnelID, scope)
	}
	return core.TunnelTokenResponse{}, nil
}
func (stubAPI) CreatePort(context.Context, string, string, core.CreateTunnelPortRequest) (core.TunnelPortResponse, error) {
	return core.TunnelPortResponse{}, nil
}
func (stubAPI) ListPorts(context.Context, string, string) ([]core.TunnelPortResponse, error) {
	return []core.TunnelPortResponse{}, nil
}
func (stubAPI) GetPort(context.Context, string, string, uint16) (core.TunnelPortResponse, error) {
	return core.TunnelPortResponse{}, nil
}
func (stubAPI) UpdatePort(context.Context, string, string, uint16, core.UpdateTunnelPortRequest) (core.TunnelPortResponse, error) {
	return core.TunnelPortResponse{}, nil
}
func (stubAPI) DeletePort(context.Context, string, string, uint16) (bool, error) {
	return true, nil
}
func (stubAPI) GetLimits(context.Context, string) (core.LimitsResponse, error) {
	return core.LimitsResponse{}, nil
}
