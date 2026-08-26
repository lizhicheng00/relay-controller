package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"mgmt-service/internal/core"
	"mgmt-service/internal/security"
	"mgmt-service/internal/store"
)

var testIdentity = core.Identity{
	AccountNamespace: "ns-a-test", Namespace: "ns-u-test",
}

var testAssertion = core.IdentityAssertion{DomainID: "domain-1", UserID: "user-1"}

type fakeRepository struct {
	identity     core.Identity
	fingerprint  core.IdentityFingerprint
	keys         []core.APIKey
	created      core.APIKey
	createdKeys  []core.NewAPIKey
	findDigest   []byte
	deletedKeyID string
	identityErr  error
	apiKeyErr    error
	listErr      error
	createErr    error
	deleteErr    error
}

func (f *fakeRepository) EnsureIdentity(
	_ context.Context,
	fingerprint core.IdentityFingerprint,
	_ core.IdentitySeed,
) (core.Identity, error) {
	f.fingerprint = fingerprint
	return f.identity, f.identityErr
}

func (f *fakeRepository) FindIdentity(
	_ context.Context,
	fingerprint core.IdentityFingerprint,
) (core.Identity, error) {
	f.fingerprint = fingerprint
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
	f.createdKeys = append(f.createdKeys, key)
	return f.created, f.createErr
}

func (f *fakeRepository) DeleteAPIKey(_ context.Context, _ string, keyID string) error {
	f.deletedKeyID = keyID
	return f.deleteErr
}

func TestCreateAPIKeyAllowsRepeatedNames(t *testing.T) {
	repository := &fakeRepository{identity: testIdentity}
	application := newTestService(repository)

	first, err := application.CreateAPIKey(
		context.Background(), testAssertion, "CLI login", core.APIKeyScopeDevBridge)
	if err != nil {
		t.Fatal(err)
	}
	second, err := application.CreateAPIKey(
		context.Background(), testAssertion, "CLI login", core.APIKeyScopeDevBridge)
	if err != nil {
		t.Fatal(err)
	}
	if first.Value == second.Value || len(repository.createdKeys) != 2 ||
		repository.createdKeys[0].ID == repository.createdKeys[1].ID {
		t.Fatalf("API keys were reused: first=%#v second=%#v", first, second)
	}
}

func TestCheckAPIKeyReturnsMappedIdentity(t *testing.T) {
	repository := &fakeRepository{identity: testIdentity}
	application := newTestService(repository)
	key, _ := security.NewAPIKey(core.APIKeyScopeDevBridge)

	result, err := application.CheckAPIKey(context.Background(), key)
	if err != nil || result.Identity != testIdentity || result.Scope != core.APIKeyScopeDevBridge {
		t.Fatalf("CheckAPIKey() = %#v, %v", result, err)
	}
	if len(repository.findDigest) != 32 {
		t.Fatalf("digest length = %d", len(repository.findDigest))
	}
}

func TestResolveIdentityEnsuresMapping(t *testing.T) {
	repository := &fakeRepository{identity: testIdentity}
	application := newTestService(repository)

	identity, err := application.ResolveIdentity(context.Background(), testAssertion)
	if err != nil || identity != testIdentity || len(repository.fingerprint.Domain) != sha256.Size ||
		len(repository.fingerprint.User) != sha256.Size ||
		bytes.Contains(repository.fingerprint.Domain, []byte(testAssertion.DomainID)) ||
		bytes.Contains(repository.fingerprint.User, []byte(testAssertion.UserID)) {
		t.Fatalf("ResolveIdentity() = %#v, fingerprint = %#v, error = %v",
			identity, repository.fingerprint, err)
	}
}

func TestCreateAPIKeyReturnsSecretOnce(t *testing.T) {
	created := core.APIKey{
		ID: "abcdefghijklmnopqrstuvwxyz", Name: "local-cli",
		Scope: core.APIKeyScopeDevBox, Mask: "devbox_abcd...1234",
		CreatedAt: time.Now(),
	}
	repository := &fakeRepository{created: created}
	application := newTestService(repository)

	issued, err := application.CreateAPIKey(
		context.Background(), testAssertion, " local-cli ", core.APIKeyScopeDevBox)
	if err != nil {
		t.Fatal(err)
	}
	if issued.APIKey != created || !strings.HasPrefix(issued.Value, "devbox_") ||
		!keyIDPattern.MatchString(repository.createdKeys[0].ID) ||
		repository.createdKeys[0].Name != "local-cli" ||
		repository.createdKeys[0].Scope != core.APIKeyScopeDevBox {
		t.Fatalf("issued key = %#v, stored = %#v", issued, repository.createdKeys[0])
	}
	_, digest, err := security.ParseAPIKey(issued.Value)
	if err != nil || !bytes.Equal(digest, repository.createdKeys[0].Digest) {
		t.Fatalf("issued digest = %x, %v", digest, err)
	}
}

func TestAPIKeyValidationAndBusinessErrors(t *testing.T) {
	application := newTestService(&fakeRepository{})
	for _, name := range []string{"", "bad\nname"} {
		if _, err := application.CreateAPIKey(
			context.Background(), testAssertion, name, core.APIKeyScopeDevBridge,
		); err == nil {
			t.Fatalf("CreateAPIKey(%q) succeeded", name)
		}
	}

	repository := &fakeRepository{identity: testIdentity, createErr: store.ErrKeyLimit}
	application = newTestService(repository)
	_, err := application.CreateAPIKey(
		context.Background(), testAssertion, "fifth", core.APIKeyScopeDevBridge)
	assertAppError(t, err, 409, core.CodeAPIKeyLimitReached)

	_, err = application.CreateAPIKey(context.Background(), testAssertion, "invalid", "unknown")
	assertAppError(t, err, 400, core.CodeParamInvalid)

	repository = &fakeRepository{identity: testIdentity}
	application = newTestService(repository)
	err = application.DeleteAPIKey(context.Background(), testAssertion, "abcdefghijklmnopqrstuvwxyz")
	if err != nil || repository.deletedKeyID != "abcdefghijklmnopqrstuvwxyz" {
		t.Fatalf("DeleteAPIKey() id = %q, error = %v", repository.deletedKeyID, err)
	}
}

func TestRejectsInvalidIdentityAndAPIKey(t *testing.T) {
	application := newTestService(&fakeRepository{})
	_, err := application.CreateAPIKey(context.Background(), core.IdentityAssertion{
		DomainID: "invalid value", UserID: "user-1",
	}, "CLI login", core.APIKeyScopeDevBridge)
	var appError *core.AppError
	if !errors.As(err, &appError) || len(appError.Details) != 1 ||
		appError.Details[0].Target != "X-Domain-Id" {
		t.Fatalf("CreateAPIKey() error = %#v", err)
	}
	_, err = application.CreateAPIKey(context.Background(), core.IdentityAssertion{
		DomainID: "domain-1", UserID: "user-1",
	}, "CLI login", "unknown")
	if !errors.As(err, &appError) || len(appError.Details) != 1 ||
		appError.Details[0].Target != "scope" {
		t.Fatalf("CreateAPIKey(scope) error = %#v", err)
	}
	if _, err := application.CheckAPIKey(context.Background(), "invalid"); err == nil {
		t.Fatal("CheckAPIKey() accepted an invalid API key")
	}
	if err := application.DeleteAPIKey(context.Background(), testAssertion, "invalid"); err == nil {
		t.Fatal("DeleteAPIKey() accepted an invalid key ID")
	}
}

func TestMissingCredentialIsUnauthorized(t *testing.T) {
	application := newTestService(&fakeRepository{apiKeyErr: store.ErrNotFound})
	key, _ := security.NewAPIKey(core.APIKeyScopeDevBridge)
	_, err := application.CheckAPIKey(context.Background(), key)
	assertAppError(t, err, 401, core.CodeUnauthorized)
}

func TestMissingIdentityReturnsEmptyList(t *testing.T) {
	application := newTestService(&fakeRepository{identityErr: store.ErrNotFound})
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

func newTestService(repository repository) *Service {
	return New(repository, testFingerprinter{})
}

type testFingerprinter struct{}

func (testFingerprinter) Fingerprint(purpose string, values ...string) []byte {
	digest := sha256.New()
	_, _ = digest.Write([]byte(purpose))
	for _, value := range values {
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(value))
	}
	return digest.Sum(nil)
}
