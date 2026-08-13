package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mgmt-service/internal/core"
	"mgmt-service/internal/security"
	"mgmt-service/internal/store"
)

var testIdentity = core.Identity{
	DomainID: "domain-1", UserID: "user-1",
	AccountNamespace: "ns-a-test", Namespace: "ns-u-test",
}

type fakeRepository struct {
	identity     core.Identity
	keys         []core.APIKey
	created      core.APIKey
	provisionKey core.NewAPIKey
	createdKey   core.NewAPIKey
	findDigest   []byte
	deletedKeyID string
	provisionErr error
	findErr      error
	listErr      error
	createErr    error
	deleteErr    error
}

func (f *fakeRepository) Provision(
	_ context.Context,
	_ core.IdentityAssertion,
	_ core.IdentitySeed,
	key core.NewAPIKey,
) (core.Identity, error) {
	f.provisionKey = key
	return f.identity, f.provisionErr
}

func (f *fakeRepository) FindIdentity(_ context.Context, digest []byte) (core.Identity, error) {
	f.findDigest = append([]byte(nil), digest...)
	return f.identity, f.findErr
}

func (f *fakeRepository) ListAPIKeys(context.Context, string) ([]core.APIKey, error) {
	return f.keys, f.listErr
}

func (f *fakeRepository) CreateAPIKey(
	_ context.Context,
	_ string,
	key core.NewAPIKey,
) (core.APIKey, error) {
	f.createdKey = key
	return f.created, f.createErr
}

func (f *fakeRepository) DeleteAPIKey(_ context.Context, _ string, keyID string) error {
	f.deletedKeyID = keyID
	return f.deleteErr
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
	if first.APIKey != second.APIKey || first.Identity != testIdentity ||
		!strings.HasPrefix(first.APIKey, "devbridge_") {
		t.Fatalf("credentials = %#v, %#v", first, second)
	}
	digest, _ := security.DigestAPIKey(first.APIKey)
	if !bytes.Equal(digest, repository.provisionKey.Digest) ||
		repository.provisionKey.Name != core.DefaultAPIKeyName {
		t.Fatalf("default key = %#v", repository.provisionKey)
	}
}

func TestAuthenticateReturnsMappedIdentity(t *testing.T) {
	repository := &fakeRepository{identity: testIdentity}
	application := New(repository, security.NewAPIKeys(strings.Repeat("s", 32)))
	key, _ := application.keys.DefaultFor("domain-1", "user-1")

	identity, err := application.Authenticate(context.Background(), key)
	if err != nil || identity != testIdentity {
		t.Fatalf("Authenticate() = %#v, %v", identity, err)
	}
	if len(repository.findDigest) != 32 {
		t.Fatalf("digest length = %d", len(repository.findDigest))
	}
}

func TestCreateAPIKeyReturnsSecretOnce(t *testing.T) {
	created := core.APIKey{
		ID: "key_abcdefghijklmnopqrstuvwxyz", Name: "local-cli",
		Scenario: core.APIKeyScenarioDevBox, Mask: "devbox_abcd...1234",
		CreatedAt: time.Now(),
	}
	repository := &fakeRepository{created: created}
	application := New(repository, security.NewAPIKeys(strings.Repeat("s", 32)))

	issued, err := application.CreateAPIKey(
		context.Background(), testIdentity, " local-cli ", core.APIKeyScenarioDevBox)
	if err != nil {
		t.Fatal(err)
	}
	if issued.APIKey != created || !strings.HasPrefix(issued.Value, "devbox_") ||
		repository.createdKey.Name != "local-cli" ||
		repository.createdKey.Scenario != core.APIKeyScenarioDevBox {
		t.Fatalf("issued key = %#v, stored = %#v", issued, repository.createdKey)
	}
	digest, err := security.DigestAPIKey(issued.Value)
	if err != nil || !bytes.Equal(digest, repository.createdKey.Digest) {
		t.Fatalf("issued digest = %x, %v", digest, err)
	}
}

func TestAPIKeyValidationAndBusinessErrors(t *testing.T) {
	application := New(&fakeRepository{}, security.NewAPIKeys(strings.Repeat("s", 32)))
	for _, name := range []string{"", "default", "bad\nname"} {
		if _, err := application.CreateAPIKey(
			context.Background(), testIdentity, name, core.APIKeyScenarioDevBridge,
		); err == nil {
			t.Fatalf("CreateAPIKey(%q) succeeded", name)
		}
	}

	repository := &fakeRepository{createErr: store.ErrKeyLimit}
	application = New(repository, security.NewAPIKeys(strings.Repeat("s", 32)))
	_, err := application.CreateAPIKey(
		context.Background(), testIdentity, "fifth", core.APIKeyScenarioDevBridge)
	assertAppError(t, err, 409, "API_KEY_LIMIT_REACHED")

	_, err = application.CreateAPIKey(context.Background(), testIdentity, "invalid", "unknown")
	assertAppError(t, err, 400, "PARAM_INVALID")

	repository = &fakeRepository{deleteErr: store.ErrDefaultKey}
	application = New(repository, security.NewAPIKeys(strings.Repeat("s", 32)))
	err = application.DeleteAPIKey(context.Background(), testIdentity, "key_abcdefghijklmnopqrstuvwxyz")
	assertAppError(t, err, 409, "DEFAULT_API_KEY")
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
	if err := application.DeleteAPIKey(context.Background(), testIdentity, "invalid"); err == nil {
		t.Fatal("DeleteAPIKey() accepted an invalid key ID")
	}
}

func TestMissingCredentialIsUnauthorized(t *testing.T) {
	application := New(&fakeRepository{findErr: store.ErrNotFound},
		security.NewAPIKeys(strings.Repeat("s", 32)))
	key, _ := application.keys.DefaultFor("domain-1", "user-1")
	_, err := application.Authenticate(context.Background(), key)
	assertAppError(t, err, 401, "UNAUTHORIZED")
}

func assertAppError(t *testing.T, err error, status int, code string) {
	t.Helper()
	var appError *core.AppError
	if !errors.As(err, &appError) || appError.Status != status || appError.Code != code {
		t.Fatalf("error = %#v, want status %d code %s", err, status, code)
	}
}
