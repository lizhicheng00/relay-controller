package service

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"mgmt-service/internal/apikey"
	"mgmt-service/internal/domain"
	"mgmt-service/internal/session"
	"mgmt-service/internal/store"
)

var testIdentity = domain.Identity{
	AccountID:        "acc_test",
	AccountNamespace: "ns-account",
	PrincipalID:      "prn_test",
	NamespaceID:      "nsp_test",
	Namespace:        "ns-user",
	IAMUserName:      "user",
}

type fakeStore struct {
	identity         domain.Identity
	newKey           domain.NewAPIKey
	credential       domain.Credential
	credentialDigest []byte
	createKeyErr     error
	touched          bool
	namespace        domain.Namespace
}

func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) Close() error               { return nil }
func (f *fakeStore) ResolveIdentity(
	context.Context, domain.IAMIdentity, domain.IdentitySeed,
) (domain.Identity, error) {
	return f.identity, nil
}
func (f *fakeStore) CreateAPIKey(
	_ context.Context, _ string, key domain.NewAPIKey, _ int,
) (domain.APIKey, error) {
	f.newKey = key
	if f.createKeyErr != nil {
		return domain.APIKey{}, f.createKeyErr
	}
	return metadata(key), nil
}
func (f *fakeStore) DeleteAPIKey(context.Context, string, string, string) error { return nil }
func (f *fakeStore) ListAPIKeys(context.Context, string, string) ([]domain.APIKey, error) {
	return []domain.APIKey{}, nil
}
func (f *fakeStore) FindCredential(_ context.Context, digest []byte) (domain.Credential, error) {
	if f.credential.APIKeyID == "" || !bytes.Equal(digest, f.credentialDigest) {
		return domain.Credential{}, store.ErrNotFound
	}
	return f.credential, nil
}
func (f *fakeStore) TouchAPIKey(context.Context, string, time.Time) error {
	f.touched = true
	return nil
}
func (f *fakeStore) CreateNamespace(
	_ context.Context, value domain.NewNamespace,
) (domain.Namespace, error) {
	f.namespace = domain.Namespace{
		ID: value.ID, Name: value.Name, DisplayName: value.DisplayName,
	}
	return f.namespace, nil
}
func (f *fakeStore) GetNamespace(context.Context, string, string) (domain.Namespace, error) {
	return f.namespace, nil
}
func (f *fakeStore) ListNamespaces(context.Context, string) ([]domain.Namespace, error) {
	return []domain.Namespace{f.namespace}, nil
}
func (f *fakeStore) UpdateNamespace(
	_ context.Context, _, _ string, displayName string,
) (domain.Namespace, error) {
	f.namespace.DisplayName = displayName
	return f.namespace, nil
}
func (f *fakeStore) DeleteNamespace(context.Context, string, string) error { return nil }

type fakeSessions struct {
	identity domain.Identity
	token    string
	expires  time.Time
	consumed bool
}

func (f *fakeSessions) Ping(context.Context) error { return nil }
func (f *fakeSessions) Close() error               { return nil }
func (f *fakeSessions) Create(
	_ context.Context, identity domain.Identity, _ time.Duration,
) (string, time.Time, error) {
	f.identity = identity
	return f.token, f.expires, nil
}
func (f *fakeSessions) Consume(_ context.Context, token string) (domain.Identity, error) {
	if token != f.token || f.consumed {
		return domain.Identity{}, session.ErrNotFound
	}
	f.consumed = true
	return f.identity, nil
}

func TestLoginAndAPIKeyIssueAreSeparate(t *testing.T) {
	repository := &fakeStore{identity: testIdentity}
	sessions := testSessions()
	application := newTestService(repository, sessions)

	login, err := application.LoginIAM(context.Background(), domain.IAMIdentity{
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
	if issued.Name != "default" || issued.Permission != domain.PermissionWrite {
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
		context.Background(), sessions.token, "automation", domain.PermissionRead)
	if err != nil {
		t.Fatalf("IssueLoginAPIKey() error = %v", err)
	}
	repository.credential = domain.Credential{
		Identity: testIdentity, APIKeyID: repository.newKey.ID, Permission: domain.PermissionRead,
	}
	repository.credentialDigest = repository.newKey.SecretHash

	auth, err := application.Authenticate(context.Background(), issued.Value)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if auth.Namespace != testIdentity.Namespace || auth.Permission != domain.PermissionRead || !repository.touched {
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
	applicationError, ok := err.(*Error)
	if !ok || applicationError.Code != "API_KEY_LIMIT" {
		t.Fatalf("IssueLoginAPIKey() error = %#v", err)
	}
}

func TestLoginValidatesIdentityBeforeStore(t *testing.T) {
	repository := &fakeStore{identity: testIdentity}
	application := newTestService(repository, testSessions())
	_, err := application.LoginIAM(context.Background(), domain.IAMIdentity{
		DomainID: "invalid value", UserID: "user-1",
	})
	applicationError, ok := err.(*Error)
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

func newTestService(repository store.Store, sessions session.Store) *Service {
	return New(
		repository,
		sessions,
		apikey.NewCodec("01234567890123456789012345678901"),
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

func metadata(key domain.NewAPIKey) domain.APIKey {
	return domain.APIKey{
		ID: key.ID, Name: key.Name, Mask: key.Mask, Permission: key.Permission,
		CreatedAt: time.Date(2026, 8, 11, 8, 0, 0, 0, time.UTC),
	}
}
