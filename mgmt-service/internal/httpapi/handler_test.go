package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mgmt-service/internal/core"
)

type fakeApplication struct {
	loginResult core.LoginSession
	issueResult core.IssuedAPIKey
	auth        core.AuthContext
	authError   error
	issueToken  string
}

func (f *fakeApplication) LoginIAM(
	context.Context, core.IAMIdentity,
) (core.LoginSession, error) {
	return f.loginResult, nil
}
func (f *fakeApplication) IssueLoginAPIKey(
	_ context.Context, token, _, _ string,
) (core.IssuedAPIKey, error) {
	f.issueToken = token
	return f.issueResult, nil
}
func (f *fakeApplication) Authenticate(context.Context, string) (core.AuthContext, error) {
	return f.auth, f.authError
}
func (f *fakeApplication) ListAPIKeys(
	context.Context, core.Identity, string,
) ([]core.APIKey, error) {
	return []core.APIKey{}, nil
}
func (f *fakeApplication) CreateAPIKey(
	context.Context, core.Identity, string, string, string,
) (core.IssuedAPIKey, error) {
	return core.IssuedAPIKey{}, nil
}
func (f *fakeApplication) DeleteAPIKey(context.Context, core.Identity, string, string) error {
	return nil
}
func (f *fakeApplication) CreateNamespace(
	context.Context, core.Identity, string,
) (core.Namespace, error) {
	return core.Namespace{ID: "nsp_new", Name: "ns-u-new", DisplayName: "New"}, nil
}
func (f *fakeApplication) GetNamespace(
	context.Context, core.Identity, string,
) (core.Namespace, error) {
	return core.Namespace{}, nil
}
func (f *fakeApplication) ListNamespaces(
	context.Context, core.Identity,
) ([]core.Namespace, error) {
	return []core.Namespace{}, nil
}
func (f *fakeApplication) UpdateNamespace(
	context.Context, core.Identity, string, string,
) (core.Namespace, error) {
	return core.Namespace{}, nil
}
func (f *fakeApplication) DeleteNamespace(context.Context, core.Identity, string) error {
	return nil
}

type readyStore struct{ err error }

func (s readyStore) Ping(context.Context) error { return s.err }

func TestLoginRejectsUntrustedIdentityHeaders(t *testing.T) {
	server := newTestServer(&fakeApplication{})
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/iam/login", nil)
	request.Header.Set("X-IAM-Domain-Id", "domain-1")
	request.Header.Set("X-IAM-User-Id", "user-1")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	assertErrorCode(t, response, "UNAUTHORIZED")
}

func TestLoginReturnsOnlyTemporaryCredential(t *testing.T) {
	application := &fakeApplication{loginResult: core.LoginSession{
		LoginToken: strings.Repeat("l", 43),
		ExpiresAt:  time.Date(2026, 8, 11, 8, 5, 0, 0, time.UTC),
		Identity: core.Identity{
			AccountNamespace: "ns-account", PrincipalID: "prn-test",
			NamespaceID: "nsp-test", Namespace: "ns-user",
		},
	}}
	server := newTestServer(application)
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/iam/login", nil)
	request.Header.Set("X-DevBridge-Proxy-Token", strings.Repeat("t", 32))
	request.Header.Set("X-IAM-Domain-Id", "domain-1")
	request.Header.Set("X-IAM-User-Id", "user-1")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status = %d, headers = %#v, body = %s", response.Code, response.Header(), response.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["loginToken"] == nil || body["apiKey"] != nil {
		t.Fatalf("response = %#v", body)
	}
}

func TestLoginTokenIsExchangedSeparately(t *testing.T) {
	application := &fakeApplication{issueResult: core.IssuedAPIKey{
		APIKey: core.APIKey{
			ID: "key-test", Name: "default", Mask: "ab...1234", Permission: "write",
		},
		Value: strings.Repeat("a", 32),
	}}
	server := newTestServer(application)
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/api-key",
		strings.NewReader(`{"name":"default","permission":"write"}`))
	request.Header.Set("Authorization", "Bearer "+strings.Repeat("l", 43))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated || application.issueToken != strings.Repeat("l", 43) {
		t.Fatalf("status = %d, token = %q, body = %s", response.Code, application.issueToken, response.Body)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

func TestReadKeyCannotMutateNamespace(t *testing.T) {
	application := &fakeApplication{auth: core.AuthContext{
		Identity: core.Identity{PrincipalID: "prn-test"}, Permission: core.PermissionRead,
	}}
	server := newTestServer(application)
	request := httptest.NewRequest(http.MethodPost, "/v1/namespaces",
		strings.NewReader(`{"displayName":"New"}`))
	request.Header.Set("X-API-Key", strings.Repeat("a", 32))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	assertErrorCode(t, response, "FORBIDDEN")
}

func TestAuthenticatedEndpointUsesAPIKeyMiddleware(t *testing.T) {
	application := &fakeApplication{authError: core.Unauthorized("X-API-Key")}
	server := newTestServer(application)
	request := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	assertErrorCode(t, response, "UNAUTHORIZED")
}

func TestReadyReportsDependencyFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New(&fakeApplication{}, []Readiness{
		readyStore{}, readyStore{err: errors.New("down")},
	}, strings.Repeat("t", 32), logger)
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	assertErrorCode(t, response, "NOT_READY")
}

func newTestServer(application API) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(application, []Readiness{readyStore{}}, strings.Repeat("t", 32), logger)
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != expected {
		t.Fatalf("error code = %q, want %q", body.Error.Code, expected)
	}
}
