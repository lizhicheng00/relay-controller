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

	"mgmt-service/internal/core"
)

type fakeAPI struct {
	provisioned core.ProvisionedCredential
	identity    core.Identity
	assertion   core.IdentityAssertion
	authError   error
}

func (f *fakeAPI) ProvisionAPIKey(
	_ context.Context,
	assertion core.IdentityAssertion,
) (core.ProvisionedCredential, error) {
	f.assertion = assertion
	return f.provisioned, nil
}

func (f *fakeAPI) Authenticate(context.Context, string) (core.Identity, error) {
	return f.identity, f.authError
}

type readyStore struct{ err error }

func (s readyStore) Ping(context.Context) error { return s.err }

func TestProvisionRequiresTrustedProxy(t *testing.T) {
	server := newTestServer(&fakeAPI{})
	request := httptest.NewRequest(http.MethodPost, "/v1/api-key", nil)
	request.Header.Set("X-Domain-Id", "domain-1")
	request.Header.Set("X-User-Id", "user-1")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	assertErrorCode(t, response, "UNAUTHORIZED")
}

func TestProvisionUsesDomainAndUser(t *testing.T) {
	application := &fakeAPI{provisioned: core.ProvisionedCredential{
		Identity: core.Identity{
			DomainID: "domain-1", UserID: "user-1",
			AccountNamespace: "ns-a-test", Namespace: "ns-u-test",
		},
		APIKey: strings.Repeat("a", 32),
	}}
	server := newTestServer(application)
	request := httptest.NewRequest(http.MethodPost, "/v1/api-key", nil)
	request.Header.Set("X-DevBridge-Proxy-Token", strings.Repeat("t", 32))
	request.Header.Set("X-Domain-Id", "domain-1")
	request.Header.Set("X-User-Id", "user-1")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status = %d, headers = %#v, body = %s", response.Code, response.Header(), response.Body)
	}
	if application.assertion.DomainID != "domain-1" || application.assertion.UserID != "user-1" {
		t.Fatalf("assertion = %#v", application.assertion)
	}
	var result core.ProvisionedCredential
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.APIKey == "" {
		t.Fatalf("response = %#v, %v", result, err)
	}
}

func TestMeUsesAPIKeyIdentity(t *testing.T) {
	identity := core.Identity{
		DomainID: "domain-1", UserID: "user-1",
		AccountNamespace: "ns-a-test", Namespace: "ns-u-test",
	}
	server := newTestServer(&fakeAPI{identity: identity})
	request := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	request.Header.Set("X-API-Key", strings.Repeat("a", 32))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	var result core.Identity
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result != identity {
		t.Fatalf("identity = %#v, %v", result, err)
	}
}

func TestReadyReportsDependencyFailure(t *testing.T) {
	server := New(&fakeAPI{}, []Readiness{readyStore{err: errors.New("down")}},
		strings.Repeat("t", 32), testLogger())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
}

func newTestServer(api API) http.Handler {
	return New(api, []Readiness{readyStore{}}, strings.Repeat("t", 32), testLogger())
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != expected {
		t.Fatalf("error code = %q, want %q", body.Error.Code, expected)
	}
}
