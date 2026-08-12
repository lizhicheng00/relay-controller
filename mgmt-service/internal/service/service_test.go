package service

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"mgmt-service/internal/core"
	"mgmt-service/internal/security"
	"mgmt-service/internal/session"
	"mgmt-service/internal/store"
)

var testIdentity = core.Identity{
	AccountID:        "acc_test",
	AccountNamespace: "ns-account",
	PrincipalID:      "prn_test",
	NamespaceID:      "nsp_test",
	Namespace:        "ns-user",
	IAMUserName:      "user",
}

type fakeStore struct {
	identity         core.Identity
	newKey           core.NewAPIKey
	credential       core.Credential
	credentialDigest []byte
	createKeyErr     error
	touched          bool
	namespace        core.Namespace
}

func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) Close() error               { return nil }
func (f *fakeStore) ResolveIdentity(
	context.Context, core.IAMIdentity, core.IdentitySeed,
) (core.Identity, error) {
	return f.identity, nil
}
func (f *fakeStore) CreateAPIKey(
	_ context.Context, _ string, key core.NewAPIKey, _ int,
) (core.APIKey, error) {
	f.newKey = key
	if f.createKeyErr != nil {
		return core.APIKey{}, f.createKeyErr
	}
	return metadata(key), nil
}
func (f *fakeStore) DeleteAPIKey(context.Context, string, string, string) error { return nil }
func (f *fakeStore) ListAPIKeys(context.Context, string, string) ([]core.APIKey, error) {
	return []core.APIKey{}, nil
}
func (f *fakeStore) FindCredential(_ context.Context, digest []byte) (core.Credential, error) {
	if f.credential.APIKeyID == "" || !bytes.Equal(digest, f.credentialDigest) {
		return core.Credential{}, store.ErrNotFound
	}
	return f.credential, nil
}
func (f *fakeStore) TouchAPIKey(context.Context, string, time.Time) error {
	f.touched = true
	return nil
}
func (f *fakeStore) CreateNamespace(
	_ context.Context, value core.NewNamespace,
) (core.Namespace, error) {
	f.namespace = core.Namespace{
		ID: value.ID, Name: value.Name, DisplayName: value.DisplayName,
	}
	return f.namespace, nil
}
func (f *fakeStore) GetNamespace(context.Context, string, string) (core.Namespace, error) {
	return f.namespace, nil
}
func (f *fakeStore) ListNamespaces(context.Context, string) ([]core.Namespace, error) {
	return []core.Namespace{f.namespace}, nil
}
func (f *fakeStore) UpdateNamespace(
	_ context.Context, _, _ string, displayName string,
) (core.Namespace, error) {
	f.namespace.DisplayName = displayName
	return f.namespace, nil
}
func (f *fakeStore) DeleteNamespace(context.Context, string, string) error { return nil }

type fakeSessions struct {
	identity core.Identity
	token    string
	expires  time.Time
	consumed bool
}

func (f *fakeSessions) Ping(context.Context) error { return nil }
func (f *fakeSessions) Close() error               { return nil }
func (f *fakeSessions) Create(
	_ context.Context, identity core.Identity, _ time.Duration,
) (string, time.Time, error) {
	f.identity = identity
	return f.token, f.expires, nil
}
func (f *fakeSessions) Consume(_ context.Context, token string) (core.Identity, error) {
	if token != f.token || f.consumed {
		return core.Identity{}, session.ErrNotFound
	}
	f.consumed = true
	return f.identity, nil
}

func TestLoginAndAPIKeyIssueAreSeparate(t *testing.T) {
	repository := &fakeStore{identity: testIdentity}
	sessions := testSessions()
	application := newTestService(repository, sessions)

	login, err := application.LoginIAM(context.Background(), core.IAMIdentity{
		DomainID: "domain-1", UserID: "user-1", UserName: "user",
	})
	if err != nil {
		t.Fatalf("LoginIAM() error = %v", err)
	}
	if login.LoginToken != sessions.token || login.Identity.Namespace != testIdentity.Namespace {
		t.Fatalf("LoginIAM() result = %#v", login)
	}
	if repository.newKey.ID != "" {
		t.Fatal("LoginIAM() issued an API key")
	}

	issued, err := application.IssueLoginAPIKey(
		context.Background(), login.LoginToken, "", "")
	if err != nil {
		t.Fatalf("IssueLoginAPIKey() error = %v", err)
	}
	if len(issued.Value) != 32 || strings.Trim(issued.Value, "abcdefghijklmnopqrstuvwxyz0123456789") != "" {
		t.Fatalf("IssueLoginAPIKey() value = %q", issued.Value)
	}
	if issued.Name != "default" || issued.Permission != core.PermissionWrite {
		t.Fatalf("IssueLoginAPIKey() metadata = %#v", issued.APIKey)
	}
	if repository.newKey.NamespaceID != testIdentity.NamespaceID ||
		bytes.Equal(repository.newKey.SecretHash, []byte(issued.Value)) {
		t.Fatalf("stored key = %#v", repository.newKey)
	}
	if _, err := application.IssueLoginAPIKey(
		context.Background(), login.LoginToken, "second", "read"); err == nil {
		t.Fatal("IssueLoginAPIKey() reused a one-time login token")
	}
}

func TestAuthenticateReturnsNamespaceAndPermission(t *testing.T) {
	repository := &fakeStore{identity: testIdentity}
	sessions := testSessions()
	application := newTestService(repository, sessions)
	issued, err := application.IssueLoginAPIKey(
		context.Background(), sessions.token, "automation", core.PermissionRead)
	if err != nil {
		t.Fatalf("IssueLoginAPIKey() error = %v", err)
	}
	repository.credential = core.Credential{
		Identity: testIdentity, APIKeyID: repository.newKey.ID, Permission: core.PermissionRead,
	}
	repository.credentialDigest = repository.newKey.SecretHash

	auth, err := application.Authenticate(context.Background(), issued.Value)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if auth.Namespace != testIdentity.Namespace || auth.Permission != core.PermissionRead || !repository.touched {
		t.Fatalf("Authenticate() = %#v, touched = %v", auth, repository.touched)
	}
	modified := issued.Value[:31] + "0"
	if modified == issued.Value {
		modified = issued.Value[:31] + "1"
	}
	if _, err := application.Authenticate(context.Background(), modified); err == nil {
		t.Fatal("Authenticate() accepted a modified API key")
	}
}

func TestIssueMapsNamespaceKeyLimit(t *testing.T) {
	repository := &fakeStore{identity: testIdentity, createKeyErr: store.ErrKeyLimit}
	sessions := testSessions()
	application := newTestService(repository, sessions)
	_, err := application.IssueLoginAPIKey(context.Background(), sessions.token, "default", "write")
	applicationError, ok := err.(*core.AppError)
	if !ok || applicationError.Code != "API_KEY_LIMIT" {
		t.Fatalf("IssueLoginAPIKey() error = %#v", err)
	}
}

func TestLoginValidatesIdentityBeforeStore(t *testing.T) {
	repository := &fakeStore{identity: testIdentity}
	application := newTestService(repository, testSessions())
	_, err := application.LoginIAM(context.Background(), core.IAMIdentity{
		DomainID: "invalid value", UserID: "user-1",
	})
	applicationError, ok := err.(*core.AppError)
	if !ok || applicationError.Target != "X-IAM-Domain-Id" {
		t.Fatalf("LoginIAM() error = %#v", err)
	}
}

func TestCreateNamespaceKeepsCanonicalNameServerManaged(t *testing.T) {
	repository := &fakeStore{identity: testIdentity}
	application := newTestService(repository, testSessions())
	created, err := application.CreateNamespace(context.Background(), testIdentity, "Production")
	if err != nil {
		t.Fatalf("CreateNamespace() error = %v", err)
	}
	if created.DisplayName != "Production" || !strings.HasPrefix(created.Name, "ns-u-") {
		t.Fatalf("CreateNamespace() = %#v", created)
	}
}

func newTestService(repository repository, sessions sessionStore) *Service {
	return New(
		repository,
		sessions,
		security.NewAPIKeyCodec("01234567890123456789012345678901"),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func testSessions() *fakeSessions {
	return &fakeSessions{
		identity: testIdentity,
		token:    strings.Repeat("l", 43),
		expires:  time.Date(2026, 8, 11, 8, 5, 0, 0, time.UTC),
	}
}

func metadata(key core.NewAPIKey) core.APIKey {
	return core.APIKey{
		ID: key.ID, Name: key.Name, Mask: key.Mask, Permission: key.Permission,
		CreatedAt: time.Date(2026, 8, 11, 8, 0, 0, 0, time.UTC),
	}
}
