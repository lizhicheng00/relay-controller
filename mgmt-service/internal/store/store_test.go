package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mgmt-service/internal/core"
)

func TestStoreIdentityNamespaceAndAPIKeyLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not configured")
	}
	repository, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repository.Close()
	ctx := context.Background()
	if err := repository.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	iam := core.IAMIdentity{
		DomainID: "domain-" + suffix, UserID: "user-" + suffix, UserName: "integration-user",
	}
	seed := core.IdentitySeed{
		AccountID: "acc_" + suffix, AccountNamespace: "ns-a-" + suffix,
		PrincipalID: "prn_" + suffix, NamespaceID: "nsp_" + suffix,
		Namespace: "ns-u-" + suffix, DisplayName: "Default",
	}
	identity, err := repository.ResolveIdentity(ctx, iam, seed)
	if err != nil {
		t.Fatalf("ResolveIdentity() error = %v", err)
	}
	repeated, err := repository.ResolveIdentity(ctx, iam, core.IdentitySeed{
		AccountID: "acc_unused", AccountNamespace: "ns-a-unused-" + suffix,
		PrincipalID: "prn_unused", NamespaceID: "nsp_unused",
		Namespace: "ns-u-unused-" + suffix, DisplayName: "Unused",
	})
	if err != nil {
		t.Fatalf("repeated ResolveIdentity() error = %v", err)
	}
	if repeated.Namespace != identity.Namespace || repeated.AccountNamespace != identity.AccountNamespace {
		t.Fatalf("repeated identity = %#v, want %#v", repeated, identity)
	}

	defaultNamespace, err := repository.GetNamespace(ctx, identity.PrincipalID, identity.NamespaceID)
	if err != nil || !defaultNamespace.Default {
		t.Fatalf("GetNamespace(default) = %#v, %v", defaultNamespace, err)
	}
	secondary, err := repository.CreateNamespace(ctx, core.NewNamespace{
		ID: "nsp_s_" + suffix, AccountID: identity.AccountID, PrincipalID: identity.PrincipalID,
		Name: "ns-u-s-" + suffix, DisplayName: "Staging",
	})
	if err != nil {
		t.Fatalf("CreateNamespace() error = %v", err)
	}
	updated, err := repository.UpdateNamespace(ctx, identity.PrincipalID, secondary.ID, "Production")
	if err != nil || updated.DisplayName != "Production" {
		t.Fatalf("UpdateNamespace() = %#v, %v", updated, err)
	}
	namespaces, err := repository.ListNamespaces(ctx, identity.PrincipalID)
	if err != nil || len(namespaces) != 2 || !namespaces[0].Default {
		t.Fatalf("ListNamespaces() = %#v, %v", namespaces, err)
	}
	if err := repository.DeleteNamespace(ctx, identity.PrincipalID, identity.NamespaceID); !errors.Is(err, ErrDefaultNamespace) {
		t.Fatalf("DeleteNamespace(default) error = %v", err)
	}

	first := integrationKey("key_a_"+suffix, secondary.ID, "automation", 1)
	created, err := repository.CreateAPIKey(ctx, identity.PrincipalID, first, 2)
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	credential, err := repository.FindCredential(ctx, first.SecretHash)
	if err != nil {
		t.Fatalf("FindCredential() error = %v", err)
	}
	if credential.APIKeyID != created.ID || credential.NamespaceID != secondary.ID ||
		credential.Permission != core.PermissionWrite {
		t.Fatalf("FindCredential() = %#v", credential)
	}
	duplicate := integrationKey("key_b_"+suffix, secondary.ID, first.Name, 2)
	if _, err := repository.CreateAPIKey(ctx, identity.PrincipalID, duplicate, 2); !errors.Is(err, ErrConflict) {
		t.Fatalf("CreateAPIKey(duplicate name) error = %v", err)
	}
	second := integrationKey("key_c_"+suffix, secondary.ID, "read-only", 3)
	second.Permission = core.PermissionRead
	if _, err := repository.CreateAPIKey(ctx, identity.PrincipalID, second, 2); err != nil {
		t.Fatalf("CreateAPIKey(second) error = %v", err)
	}
	third := integrationKey("key_d_"+suffix, secondary.ID, "third", 4)
	if _, err := repository.CreateAPIKey(ctx, identity.PrincipalID, third, 2); !errors.Is(err, ErrKeyLimit) {
		t.Fatalf("CreateAPIKey(over limit) error = %v", err)
	}
	keys, err := repository.ListAPIKeys(ctx, identity.PrincipalID, secondary.ID)
	if err != nil || len(keys) != 2 {
		t.Fatalf("ListAPIKeys() = %#v, %v", keys, err)
	}
	if bytes.Contains(first.SecretHash, []byte(created.Mask)) {
		t.Fatal("stored hash contains the API key mask")
	}
	if err := repository.DeleteAPIKey(ctx, identity.PrincipalID, secondary.ID, first.ID); err != nil {
		t.Fatalf("DeleteAPIKey() error = %v", err)
	}
	if _, err := repository.FindCredential(ctx, first.SecretHash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindCredential(deleted) error = %v", err)
	}
	if err := repository.DeleteNamespace(ctx, identity.PrincipalID, secondary.ID); err != nil {
		t.Fatalf("DeleteNamespace() error = %v", err)
	}
	if _, err := repository.GetNamespace(ctx, identity.PrincipalID, secondary.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetNamespace(deleted) error = %v", err)
	}

	concurrent, err := repository.CreateNamespace(ctx, core.NewNamespace{
		ID: "nsp_c_" + suffix, AccountID: identity.AccountID, PrincipalID: identity.PrincipalID,
		Name: "ns-u-c-" + suffix, DisplayName: "Concurrent",
	})
	if err != nil {
		t.Fatalf("CreateNamespace(concurrent) error = %v", err)
	}
	assertConcurrentKeyLimit(t, repository, identity, concurrent, suffix)
	if err := repository.DeleteNamespace(ctx, identity.PrincipalID, concurrent.ID); err != nil {
		t.Fatalf("DeleteNamespace(concurrent) error = %v", err)
	}
}

func integrationKey(id, namespaceID, name string, hashByte byte) core.NewAPIKey {
	return core.NewAPIKey{
		ID: id, NamespaceID: namespaceID, Name: name, Mask: "ab...1234",
		SecretHash: bytes.Repeat([]byte{hashByte}, 32), Permission: core.PermissionWrite,
	}
}

func assertConcurrentKeyLimit(
	t *testing.T,
	repository *Store,
	identity core.Identity,
	namespace core.Namespace,
	suffix string,
) {
	t.Helper()
	const attempts = 10
	start := make(chan struct{})
	errorsFound := make(chan error, attempts)
	var successes atomic.Int32
	var wait sync.WaitGroup
	for index := range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			key := integrationKey(
				"key_x"+strconv.Itoa(index)+"_"+suffix,
				namespace.ID,
				"concurrent-"+strconv.Itoa(index),
				byte(index+20),
			)
			if _, err := repository.CreateAPIKey(
				context.Background(), identity.PrincipalID, key, 5,
			); err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrKeyLimit) {
				errorsFound <- err
			}
		}()
	}
	close(start)
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("concurrent CreateAPIKey() error = %v", err)
	}
	if successes.Load() != 5 {
		t.Fatalf("concurrent CreateAPIKey() successes = %d, want 5", successes.Load())
	}
	keys, err := repository.ListAPIKeys(context.Background(), identity.PrincipalID, namespace.ID)
	if err != nil || len(keys) != 5 {
		t.Fatalf("ListAPIKeys(concurrent) count = %d, error = %v", len(keys), err)
	}
}
