package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientResolvesAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != checkPath || request.Header.Get("X-API-Key") != "devbridge_test" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"accountNamespace":"ns-account","namespace":"ns-user","scope":"devbridge"}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := client.ResolveAPIKey(context.Background(), "devbridge_test")
	if err != nil || principal.Namespace != "ns-user" || principal.AccountNamespace != "ns-account" || principal.Scope != "devbridge" {
		t.Fatalf("ResolveAPIKey() = %#v, %v", principal, err)
	}
}

func TestClientMapsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ResolveAPIKey(context.Background(), "invalid"); err != ErrUnauthorized {
		t.Fatalf("ResolveAPIKey() error = %v", err)
	}
}

func TestClientRejectsInvalidURL(t *testing.T) {
	if _, err := NewClient("localhost:8444"); err == nil {
		t.Fatal("NewClient() accepted a URL without a scheme")
	}
}
