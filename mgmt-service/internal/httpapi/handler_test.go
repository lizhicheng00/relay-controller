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
	defaultKey   core.DefaultAPIKeyCredential
	identity     core.Identity
	keys         []core.APIKey
	issued       core.IssuedAPIKey
	assertion    core.IdentityAssertion
	createdName  string
	issuedScope  core.APIKeyScope
	createdScope core.APIKeyScope
	deletedID    string
	checkedKey   string
	authError    error
	issueError   error
	listError    error
	createError  error
	deleteError  error
}

func (f *fakeAPI) IssueDefaultAPIKey(
	_ context.Context,
	assertion core.IdentityAssertion,
	scope core.APIKeyScope,
) (core.DefaultAPIKeyCredential, error) {
	f.assertion = assertion
	f.issuedScope = scope
	return f.defaultKey, f.issueError
}

func (f *fakeAPI) CheckAPIKey(_ context.Context, value string) (core.Identity, error) {
	f.checkedKey = value
	return f.identity, f.authError
}

func (f *fakeAPI) ListAPIKeys(
	_ context.Context,
	assertion core.IdentityAssertion,
) ([]core.APIKey, error) {
	f.assertion = assertion
	return f.keys, f.listError
}

func (f *fakeAPI) CreateAPIKey(
	_ context.Context,
	assertion core.IdentityAssertion,
	name string,
	scope core.APIKeyScope,
) (core.IssuedAPIKey, error) {
	f.assertion = assertion
	f.createdName = name
	f.createdScope = scope
	return f.issued, f.createError
}

func (f *fakeAPI) DeleteAPIKey(
	_ context.Context,
	assertion core.IdentityAssertion,
	keyID string,
) error {
	f.assertion = assertion
	f.deletedID = keyID
	return f.deleteError
}

func TestIssueDefaultAPIKeyUsesDomainUserAndType(t *testing.T) {
	application := &fakeAPI{defaultKey: core.DefaultAPIKeyCredential{
		Identity: core.Identity{
			DomainID: "domain-1", UserID: "user-1",
			AccountNamespace: "ns-a-test", Namespace: "ns-u-test",
		},
		Scope: core.APIKeyScopeDevBox, APIKey: strings.Repeat("a", 32),
	}}
	server := newTestServer(application)
	request := httptest.NewRequest(http.MethodPost, apiBase+"/api-keys/default",
		strings.NewReader(`{"scope":"devbox"}`))
	request.Header.Set("X-Domain-Id", "domain-1")
	request.Header.Set("X-User-Id", "user-1")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status = %d, headers = %#v, body = %s", response.Code, response.Header(), response.Body)
	}
	if application.assertion.DomainID != "domain-1" || application.assertion.UserID != "user-1" {
		t.Fatalf("assertion = %#v", application.assertion)
	}
	if application.issuedScope != core.APIKeyScopeDevBox {
		t.Fatalf("scope = %q", application.issuedScope)
	}
	var result core.DefaultAPIKeyCredential
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.APIKey == "" {
		t.Fatalf("response = %#v, %v", result, err)
	}
}

func TestValidationErrorUsesRelayFormat(t *testing.T) {
	server := newTestServer(&fakeAPI{issueError: core.Invalid("X-User-Id", "user ID is invalid")})
	request := httptest.NewRequest(http.MethodPost, apiBase+"/api-keys/default",
		strings.NewReader(`{"scope":"devbridge"}`))
	request.Header.Set("Content-Type", "application/json")
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

func TestCheckAPIKeyReturnsIdentity(t *testing.T) {
	identity := core.Identity{
		DomainID: "domain-1", UserID: "user-1",
		AccountNamespace: "ns-a-test", Namespace: "ns-u-test",
	}
	application := &fakeAPI{identity: identity}
	server := newTestServer(application)
	request := httptest.NewRequest(http.MethodPost, apiBase+"/api-keys/check", nil)
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
	if application.checkedKey != strings.Repeat("a", 32) {
		t.Fatalf("checked key = %q", application.checkedKey)
	}
}

func TestCheckAPIKeyRejectsInvalidKey(t *testing.T) {
	server := newTestServer(&fakeAPI{authError: core.Unauthorized("X-API-Key")})
	request := httptest.NewRequest(http.MethodPost, apiBase+"/api-keys/check", nil)
	request.Header.Set("X-API-Key", "invalid")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	assertErrorCode(t, response, core.CodeUnauthorized)
}

func TestAPIKeyManagementRoutes(t *testing.T) {
	application := &fakeAPI{
		keys: []core.APIKey{{
			ID: "key_default", Name: "default",
			Scope: core.APIKeyScopeDevBridge, Default: true,
		}},
		issued: core.IssuedAPIKey{
			APIKey: core.APIKey{
				ID: "key_created", Name: "local-cli", Scope: core.APIKeyScopeDevBox,
			},
			Value: "devbox_" + strings.Repeat("a", 32),
		},
	}
	server := newTestServer(application)

	listRequest := httptest.NewRequest(http.MethodGet, apiBase+"/api-keys", nil)
	setIdentityHeaders(listRequest)
	listResponse := httptest.NewRecorder()
	server.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "key_default") {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body)
	}

	createRequest := httptest.NewRequest(http.MethodPost, apiBase+"/api-keys",
		strings.NewReader(`{"name":"local-cli","scope":"devbox"}`))
	setIdentityHeaders(createRequest)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	server.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated || application.createdName != "local-cli" ||
		application.createdScope != core.APIKeyScopeDevBox ||
		!strings.Contains(createResponse.Body.String(), "devbox_"+strings.Repeat("a", 32)) {
		t.Fatalf("create status = %d, name = %q, body = %s",
			createResponse.Code, application.createdName, createResponse.Body)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, apiBase+"/api-keys/key_created", nil)
	setIdentityHeaders(deleteRequest)
	deleteResponse := httptest.NewRecorder()
	server.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || application.deletedID != "key_created" {
		t.Fatalf("delete status = %d, key = %q", deleteResponse.Code, application.deletedID)
	}
	if application.assertion != (core.IdentityAssertion{DomainID: "domain-1", UserID: "user-1"}) {
		t.Fatalf("identity assertion = %#v", application.assertion)
	}
}

func TestCreateAPIKeyRejectsInvalidJSON(t *testing.T) {
	server := newTestServer(&fakeAPI{})
	request := httptest.NewRequest(http.MethodPost, apiBase+"/api-keys",
		strings.NewReader(`{"name":"cli","scope":"devbridge","unknown":true}`))
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
	return New(api, testLogger())
}

func setIdentityHeaders(request *http.Request) {
	request.Header.Set("X-Domain-Id", "domain-1")
	request.Header.Set("X-User-Id", "user-1")
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
