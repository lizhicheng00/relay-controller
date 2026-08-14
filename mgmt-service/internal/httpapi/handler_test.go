package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mgmt-service/internal/core"
)

type fakeAPI struct {
	provisioned     core.ProvisionedCredential
	identity        core.Identity
	keys            []core.APIKey
	issued          core.IssuedAPIKey
	assertion       core.IdentityAssertion
	createdName     string
	createdScenario core.APIKeyScenario
	deletedID       string
	authError       error
	provisionError  error
	listError       error
	createError     error
	deleteError     error
}

func (f *fakeAPI) ProvisionAPIKey(
	_ context.Context,
	assertion core.IdentityAssertion,
) (core.ProvisionedCredential, error) {
	f.assertion = assertion
	return f.provisioned, f.provisionError
}

func (f *fakeAPI) Authenticate(context.Context, string) (core.Identity, error) {
	return f.identity, f.authError
}

func (f *fakeAPI) ListAPIKeys(context.Context, core.Identity) ([]core.APIKey, error) {
	return f.keys, f.listError
}

func (f *fakeAPI) CreateAPIKey(
	_ context.Context,
	_ core.Identity,
	name string,
	scenario core.APIKeyScenario,
) (core.IssuedAPIKey, error) {
	f.createdName = name
	f.createdScenario = scenario
	return f.issued, f.createError
}

func (f *fakeAPI) DeleteAPIKey(_ context.Context, _ core.Identity, keyID string) error {
	f.deletedID = keyID
	return f.deleteError
}

func TestProvisionRequiresTrustedProxy(t *testing.T) {
	server := newTestServer(&fakeAPI{})
	request := httptest.NewRequest(http.MethodPost, apiBase+"/api-key", nil)
	request.Header.Set("X-Domain-Id", "domain-1")
	request.Header.Set("X-User-Id", "user-1")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	assertErrorCode(t, response, core.CodeUnauthorized)
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
	request := httptest.NewRequest(http.MethodPost, apiBase+"/api-key", nil)
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

func TestValidationErrorUsesRelayFormat(t *testing.T) {
	server := newTestServer(&fakeAPI{provisionError: core.Invalid("X-User-Id", "user ID is invalid")})
	request := httptest.NewRequest(http.MethodPost, apiBase+"/api-key", nil)
	request.Header.Set("X-DevBridge-Proxy-Token", strings.Repeat("t", 32))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	var result core.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadRequest || result.Error.Code != core.CodeParamInvalid ||
		len(result.Error.Details) != 1 || result.Error.Details[0].Target != "X-User-Id" {
		t.Fatalf("status = %d, response = %#v", response.Code, result)
	}
}

func TestMeUsesAPIKeyIdentity(t *testing.T) {
	identity := core.Identity{
		DomainID: "domain-1", UserID: "user-1",
		AccountNamespace: "ns-a-test", Namespace: "ns-u-test",
	}
	server := newTestServer(&fakeAPI{identity: identity})
	request := httptest.NewRequest(http.MethodGet, apiBase+"/me", nil)
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

func TestAPIKeyManagementRoutes(t *testing.T) {
	identity := core.Identity{Namespace: "ns-u-test"}
	application := &fakeAPI{
		identity: identity,
		keys: []core.APIKey{{
			ID: "key_default", Name: "default",
			Scenario: core.APIKeyScenarioDevBridge, Default: true,
		}},
		issued: core.IssuedAPIKey{
			APIKey: core.APIKey{
				ID: "key_created", Name: "local-cli", Scenario: core.APIKeyScenarioDevBox,
			},
			Value: "devbox_" + strings.Repeat("a", 32),
		},
	}
	server := newTestServer(application)

	listRequest := httptest.NewRequest(http.MethodGet, apiBase+"/api-keys", nil)
	listRequest.Header.Set("X-API-Key", strings.Repeat("a", 32))
	listResponse := httptest.NewRecorder()
	server.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "key_default") {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body)
	}

	createRequest := httptest.NewRequest(http.MethodPost, apiBase+"/api-keys",
		strings.NewReader(`{"name":"local-cli","scenario":"devbox"}`))
	createRequest.Header.Set("X-API-Key", strings.Repeat("a", 32))
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	server.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated || application.createdName != "local-cli" ||
		application.createdScenario != core.APIKeyScenarioDevBox ||
		!strings.Contains(createResponse.Body.String(), "devbox_"+strings.Repeat("a", 32)) {
		t.Fatalf("create status = %d, name = %q, body = %s",
			createResponse.Code, application.createdName, createResponse.Body)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, apiBase+"/api-keys/key_created", nil)
	deleteRequest.Header.Set("X-API-Key", strings.Repeat("a", 32))
	deleteResponse := httptest.NewRecorder()
	server.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || application.deletedID != "key_created" {
		t.Fatalf("delete status = %d, key = %q", deleteResponse.Code, application.deletedID)
	}
}

func TestCreateAPIKeyRejectsInvalidJSON(t *testing.T) {
	server := newTestServer(&fakeAPI{})
	request := httptest.NewRequest(http.MethodPost, apiBase+"/api-keys",
		strings.NewReader(`{"name":"cli","scenario":"devbridge","unknown":true}`))
	request.Header.Set("X-API-Key", strings.Repeat("a", 32))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	assertErrorCode(t, response, core.CodeParamInvalid)
}

func TestHealthRoutesAreNotExposed(t *testing.T) {
	server := newTestServer(&fakeAPI{})
	for _, path := range []string{"/healthz", "/readyz"} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d", path, response.Code)
		}
	}
}

func newTestServer(api API) http.Handler {
	return New(api, strings.Repeat("t", 32), testLogger())
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
