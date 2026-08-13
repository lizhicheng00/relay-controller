package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"mgmt-service/internal/core"
	"mgmt-service/internal/security"
	"mgmt-service/internal/store"
)

var testIdentity = core.Identity{
	DomainID: "domain-1", UserID: "user-1",
	AccountNamespace: "ns-a-test", Namespace: "ns-u-test",
}

type fakeRepository struct {
	identity        core.Identity
	provisionDigest []byte
	findDigest      []byte
	provisionErr    error
	findErr         error
}

func (f *fakeRepository) Provision(
	_ context.Context,
	_ core.IdentityAssertion,
	_ core.IdentitySeed,
	digest []byte,
) (core.Identity, error) {
	f.provisionDigest = append([]byte(nil), digest...)
	return f.identity, f.provisionErr
}

func (f *fakeRepository) FindIdentity(_ context.Context, digest []byte) (core.Identity, error) {
	f.findDigest = append([]byte(nil), digest...)
	return f.identity, f.findErr
}

func TestProvisionAPIKeyIsStable(t *testing.T) {
	repository := &fakeRepository{identity: testIdentity}
	application := New(repository, security.NewAPIKeys(strings.Repeat("s", 32)))
	assertion := core.IdentityAssertion{DomainID: "domain-1", UserID: "user-1"}

	first, err := application.ProvisionAPIKey(context.Background(), assertion)
	if err != nil {
		t.Fatalf("ProvisionAPIKey() error = %v", err)
	}
	second, err := application.ProvisionAPIKey(context.Background(), assertion)
	if err != nil {
		t.Fatalf("second ProvisionAPIKey() error = %v", err)
	}
	if first.APIKey != second.APIKey || first.Identity != testIdentity || len(first.APIKey) != 32 {
		t.Fatalf("credentials = %#v, %#v", first, second)
	}
	digest, _ := security.DigestAPIKey(first.APIKey)
	if !bytes.Equal(digest, repository.provisionDigest) {
		t.Fatal("repository received an unexpected API key digest")
	}
}

func TestAuthenticateReturnsMappedIdentity(t *testing.T) {
	repository := &fakeRepository{identity: testIdentity}
	application := New(repository, security.NewAPIKeys(strings.Repeat("s", 32)))
	key, _ := application.keys.For("domain-1", "user-1")

	identity, err := application.Authenticate(context.Background(), key)
	if err != nil || identity != testIdentity {
		t.Fatalf("Authenticate() = %#v, %v", identity, err)
	}
	if len(repository.findDigest) != 32 {
		t.Fatalf("digest length = %d", len(repository.findDigest))
	}
}

func TestRejectsInvalidIdentityAndAPIKey(t *testing.T) {
	application := New(&fakeRepository{}, security.NewAPIKeys(strings.Repeat("s", 32)))
	_, err := application.ProvisionAPIKey(context.Background(), core.IdentityAssertion{
		DomainID: "invalid value", UserID: "user-1",
	})
	var appError *core.AppError
	if !errors.As(err, &appError) || appError.Target != "X-Domain-Id" {
		t.Fatalf("ProvisionAPIKey() error = %#v", err)
	}
	if _, err := application.Authenticate(context.Background(), "invalid"); err == nil {
		t.Fatal("Authenticate() accepted an invalid API key")
	}
}

func TestMissingCredentialIsUnauthorized(t *testing.T) {
	application := New(&fakeRepository{findErr: store.ErrNotFound},
		security.NewAPIKeys(strings.Repeat("s", 32)))
	key, _ := application.keys.For("domain-1", "user-1")
	_, err := application.Authenticate(context.Background(), key)
	var appError *core.AppError
	if !errors.As(err, &appError) || appError.Status != 401 {
		t.Fatalf("Authenticate() error = %#v", err)
	}
}
