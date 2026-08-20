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

var testAssertion = core.IdentityAssertion{DomainID: "domain-1", UserID: "user-1"}

type fakeRepository struct {
	identity     core.Identity
	keys         []core.APIKey
	created      core.APIKey
	defaultKey   core.NewAPIKey
	createdKey   core.NewAPIKey
	findDigest   []byte
	deletedKeyID string
	issueErr     error
	identityErr  error
	apiKeyErr    error
	listErr      error
	createErr    error
	deleteErr    error
}

func (f *fakeRepository) IssueDefaultAPIKey(
	_ context.Context,
	_ core.IdentityAssertion,
	_ core.IdentitySeed,
	key core.NewAPIKey,
) (core.Identity, error) {
	f.defaultKey = key
	return f.identity, f.issueErr
}

func (f *fakeRepository) EnsureIdentity(
	_ context.Context,
	_ core.IdentityAssertion,
	_ core.IdentitySeed,
) (core.Identity, error) {
	return f.identity, f.identityErr
}

func (f *fakeRepository) FindIdentity(
	_ context.Context,
	_ core.IdentityAssertion,
) (core.Identity, error) {
	return f.identity, f.identityErr
}

func (f *fakeRepository) FindIdentityByAPIKey(
	_ context.Context,
	digest []byte,
) (core.Identity, error) {
	f.findDigest = append([]byte(nil), digest...)
	return f.identity, f.apiKeyErr
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

func TestIssueDefaultAPIKeyRotates(t *testing.T) {
	repository := &fakeRepository{identity: testIdentity}
	application := New(repository)
	assertion := testAssertion

	first, err := application.IssueDefaultAPIKey(context.Background(), assertion, core.APIKeyScopeDevBridge)
	if err != nil {
		t.Fatalf("IssueDefaultAPIKey() error = %v", err)
	}
	second, err := application.IssueDefaultAPIKey(context.Background(), assertion, core.APIKeyScopeDevBridge)
	if err != nil {
		t.Fatalf("second IssueDefaultAPIKey() error = %v", err)
	}
	if first.APIKey == second.APIKey || first.Identity != testIdentity || second.Identity != testIdentity ||
		!strings.HasPrefix(first.APIKey, "devbridge_") {
		t.Fatalf("credentials = %#v, %#v", first, second)
	}
	_, digest, _ := security.ParseAPIKey(second.APIKey)
	if !bytes.Equal(digest, repository.defaultKey.Digest) ||
		!keyIDPattern.MatchString(repository.defaultKey.ID) ||
		repository.defaultKey.Name != core.DefaultAPIKeyName ||
		repository.defaultKey.Scope != core.APIKeyScopeDevBridge {
		t.Fatalf("default key = %#v", repository.defaultKey)
	}
}

func TestCheckAPIKeyReturnsMappedIdentity(t *testing.T) {
	repository := &fakeRepository{identity: testIdentity}
	application := New(repository)
	key, _ := security.NewAPIKey(core.APIKeyScopeDevBridge)

	result, err := application.CheckAPIKey(context.Background(), key)
	if err != nil || result.Identity != testIdentity || result.Scope != core.APIKeyScopeDevBridge {
		t.Fatalf("CheckAPIKey() = %#v, %v", result, err)
	}
	if len(repository.findDigest) != 32 {
		t.Fatalf("digest length = %d", len(repository.findDigest))
	}
}

func TestCreateAPIKeyReturnsSecretOnce(t *testing.T) {
	created := core.APIKey{
		ID: "abcdefghijklmnopqrstuvwxyz", Name: "local-cli",
		Scope: core.APIKeyScopeDevBox, Mask: "devbox_abcd...1234",
		CreatedAt: time.Now(),
	}
	repository := &fakeRepository{created: created}
	application := New(repository)

	issued, err := application.CreateAPIKey(
		context.Background(), testAssertion, " local-cli ", core.APIKeyScopeDevBox)
	if err != nil {
		t.Fatal(err)
	}
	if issued.APIKey != created || !strings.HasPrefix(issued.Value, "devbox_") ||
		!keyIDPattern.MatchString(repository.createdKey.ID) ||
		repository.createdKey.Name != "local-cli" ||
		repository.createdKey.Scope != core.APIKeyScopeDevBox {
		t.Fatalf("issued key = %#v, stored = %#v", issued, repository.createdKey)
	}
	_, digest, err := security.ParseAPIKey(issued.Value)
	if err != nil || !bytes.Equal(digest, repository.createdKey.Digest) {
		t.Fatalf("issued digest = %x, %v", digest, err)
	}
}

func TestAPIKeyValidationAndBusinessErrors(t *testing.T) {
	application := New(&fakeRepository{})
	for _, name := range []string{"", "default", "bad\nname"} {
		if _, err := application.CreateAPIKey(
			context.Background(), testAssertion, name, core.APIKeyScopeDevBridge,
		); err == nil {
			t.Fatalf("CreateAPIKey(%q) succeeded", name)
		}
	}

	repository := &fakeRepository{identity: testIdentity, createErr: store.ErrKeyLimit}
	application = New(repository)
	_, err := application.CreateAPIKey(
		context.Background(), testAssertion, "fifth", core.APIKeyScopeDevBridge)
	assertAppError(t, err, 409, core.CodeAPIKeyLimitReached)

	_, err = application.CreateAPIKey(context.Background(), testAssertion, "invalid", "unknown")
	assertAppError(t, err, 400, core.CodeParamInvalid)

	repository = &fakeRepository{identity: testIdentity, deleteErr: store.ErrDefaultKey}
	application = New(repository)
	err = application.DeleteAPIKey(context.Background(), testAssertion, "abcdefghijklmnopqrstuvwxyz")
	assertAppError(t, err, 409, core.CodeDefaultAPIKey)
}

func TestRejectsInvalidIdentityAndAPIKey(t *testing.T) {
	application := New(&fakeRepository{})
	_, err := application.IssueDefaultAPIKey(context.Background(), core.IdentityAssertion{
		DomainID: "invalid value", UserID: "user-1",
	}, core.APIKeyScopeDevBridge)
	var appError *core.AppError
	if !errors.As(err, &appError) || len(appError.Details) != 1 ||
		appError.Details[0].Target != "X-Domain-Id" {
		t.Fatalf("IssueDefaultAPIKey() error = %#v", err)
	}
	_, err = application.IssueDefaultAPIKey(context.Background(), core.IdentityAssertion{
		DomainID: "domain-1", UserID: "user-1",
	}, "unknown")
	if !errors.As(err, &appError) || len(appError.Details) != 1 ||
		appError.Details[0].Target != "scope" {
		t.Fatalf("IssueDefaultAPIKey(scope) error = %#v", err)
	}
	if _, err := application.CheckAPIKey(context.Background(), "invalid"); err == nil {
		t.Fatal("CheckAPIKey() accepted an invalid API key")
	}
	if err := application.DeleteAPIKey(context.Background(), testAssertion, "invalid"); err == nil {
		t.Fatal("DeleteAPIKey() accepted an invalid key ID")
	}
}

func TestMissingCredentialIsUnauthorized(t *testing.T) {
	application := New(&fakeRepository{apiKeyErr: store.ErrNotFound})
	key, _ := security.NewAPIKey(core.APIKeyScopeDevBridge)
	_, err := application.CheckAPIKey(context.Background(), key)
	assertAppError(t, err, 401, core.CodeUnauthorized)
}

func TestMissingIdentityReturnsEmptyList(t *testing.T) {
	application := New(&fakeRepository{identityErr: store.ErrNotFound})
	keys, err := application.ListAPIKeys(context.Background(), testAssertion)
	if err != nil || keys == nil || len(keys) != 0 {
		t.Fatalf("ListAPIKeys() = %#v, %v", keys, err)
	}
}

func assertAppError(t *testing.T, err error, status int, code string) {
	t.Helper()
	var appError *core.AppError
	if !errors.As(err, &appError) || appError.Status != status || appError.Code != code {
		t.Fatalf("error = %#v, want status %d code %s", err, status, code)
	}
}
