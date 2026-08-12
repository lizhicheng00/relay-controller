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

	"github.com/lizhicheng00/relay-controller/relay-controller/internal/core"
)

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

func TestMissingNamespaceReturnsStructured401(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, apiBase+"/tunnels", strings.NewReader(`{"name":"dev"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	New(stubAPI{}, testLogger(), nil).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":"40100"`) || !strings.Contains(response.Body.String(), `"target":"X-Namespace"`) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
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
	request.Header.Set("X-Namespace", "ns-user-001")
	request.Header.Set("X-Account-Namespace", "ns-user-001")
	response := httptest.NewRecorder()
	New(stubAPI{}, testLogger(), nil).ServeHTTP(response, request)
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

func serve(t *testing.T, api API, method, target, body string, headers bool) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if headers {
		request.Header.Set("X-Namespace", "ns-sub-user-001")
		request.Header.Set("X-Account-Namespace", "ns-user-001")
	}
	response := httptest.NewRecorder()
	New(api, testLogger(), nil).ServeHTTP(response, request)
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
