package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"mgmt-service/internal/core"
	"mgmt-service/migrations"
)

func TestIdentityAndAPIKeyLifecycle(t *testing.T) {
	repository := openTestStore(t)
	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	assertion := core.IdentityAssertion{DomainID: "domain-" + suffix, UserID: "user-a-" + suffix}
	fingerprint := testIdentityFingerprint(assertion)
	identity, err := repository.EnsureIdentity(ctx, fingerprint, identitySeed(suffix, "a"))
	if err != nil {
		t.Fatal(err)
	}

	firstLoginKey := newTestKey("login_a_"+suffix, "CLI login")
	secondLoginKey := newTestKey("login_b_"+suffix, "CLI login")
	for _, key := range []core.NewAPIKey{firstLoginKey, secondLoginKey} {
		created, err := repository.CreateAPIKey(ctx, identity.Namespace, key)
		if err != nil || created.Name != "CLI login" {
			t.Fatalf("CreateAPIKey(CLI login) = %#v, %v", created, err)
		}
	}

	if found, err := repository.FindIdentity(ctx, fingerprint); err != nil || found != identity {
		t.Fatalf("FindIdentity() = %#v, %v", found, err)
	}
	if found, err := repository.FindIdentityByAPIKey(ctx, firstLoginKey.Digest); err != nil || found != identity {
		t.Fatalf("FindIdentityByAPIKey() = %#v, %v", found, err)
	}

	for index := 2; index < core.MaxAPIKeysPerScope; index++ {
		key := newTestKey(fmt.Sprintf("bridge_%02d_%s", index, suffix), "repeated-name")
		if _, err := repository.CreateAPIKey(ctx, identity.Namespace, key); err != nil {
			t.Fatalf("CreateAPIKey(%d) error = %v", index, err)
		}
	}
	if _, err := repository.CreateAPIKey(ctx, identity.Namespace,
		newTestKey("overflow_"+suffix, "overflow")); !errors.Is(err, ErrKeyLimit) {
		t.Fatalf("overflow error = %v", err)
	}

	for index := 0; index < core.MaxAPIKeysPerScope; index++ {
		key := newTestKey(fmt.Sprintf("devbox_%02d_%s", index, suffix), "repeated-name")
		key.Scope = core.APIKeyScopeDevBox
		key.Mask = "devbox_test...mask"
		if _, err := repository.CreateAPIKey(ctx, identity.Namespace, key); err != nil {
			t.Fatalf("CreateAPIKey(devbox %d) error = %v", index, err)
		}
	}

	keys, err := repository.ListAPIKeys(ctx, identity.Namespace)
	if err != nil || len(keys) != core.MaxAPIKeysPerScope*2 ||
		keysWithScope(keys, core.APIKeyScopeDevBridge) != core.MaxAPIKeysPerScope ||
		keysWithScope(keys, core.APIKeyScopeDevBox) != core.MaxAPIKeysPerScope {
		t.Fatalf("ListAPIKeys() count=%d error=%v", len(keys), err)
	}
	if keyByID(keys, firstLoginKey.ID).LastUsedAt == nil {
		t.Fatal("authenticated key has no last-used time")
	}

	secondIdentity, err := repository.EnsureIdentity(ctx, testIdentityFingerprint(core.IdentityAssertion{
		DomainID: assertion.DomainID, UserID: "user-b-" + suffix,
	}), identitySeed(suffix, "b"))
	if err != nil || secondIdentity.AccountNamespace != identity.AccountNamespace ||
		secondIdentity.Namespace == identity.Namespace {
		t.Fatalf("second identity = %#v, %v", secondIdentity, err)
	}
	if _, err := repository.db.ExecContext(ctx, `
		INSERT INTO api_key (id, namespace, name, key_scope, key_mask, key_hash)
		VALUES (?, ?, 'invalid-scope', 'unknown', 'unknown_test...mask', ?)`,
		"invalid_"+suffix, secondIdentity.Namespace, bytes.Repeat([]byte{98}, 32)); err == nil {
		t.Fatal("database accepted an unknown API key scope")
	}
	if err := repository.DeleteAPIKey(ctx, secondIdentity.Namespace, firstLoginKey.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-namespace delete error = %v", err)
	}
	if err := repository.DeleteAPIKey(ctx, identity.Namespace, firstLoginKey.ID); err != nil {
		t.Fatalf("DeleteAPIKey() error = %v", err)
	}
	if _, err := repository.FindIdentityByAPIKey(ctx, firstLoginKey.Digest); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted key authentication error = %v", err)
	}
	if _, err := repository.CreateAPIKey(ctx, identity.Namespace,
		newTestKey("replacement_"+suffix, "repeated-name")); err != nil {
		t.Fatalf("CreateAPIKey(after delete) error = %v", err)
	}
}

func TestConcurrentAPIKeyLimit(t *testing.T) {
	repository := openTestStore(t)
	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	identity, err := repository.EnsureIdentity(ctx,
		testIdentityFingerprint(core.IdentityAssertion{
			DomainID: "concurrent-domain-" + suffix, UserID: "user-" + suffix,
		}),
		identitySeed(suffix, "concurrent"))
	if err != nil {
		t.Fatal(err)
	}

	const attempts = 25
	start := make(chan struct{})
	results := make(chan error, attempts)
	var group sync.WaitGroup
	for index := 0; index < attempts; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			key := newTestKey(fmt.Sprintf("concurrent_%02d_%s", index, suffix), "CLI login")
			_, err := repository.CreateAPIKey(ctx, identity.Namespace, key)
			results <- err
		}(index)
	}
	close(start)
	group.Wait()
	close(results)

	succeeded, limited := 0, 0
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrKeyLimit):
			limited++
		default:
			t.Fatalf("unexpected concurrent creation error: %v", err)
		}
	}
	if succeeded != core.MaxAPIKeysPerScope || limited != attempts-core.MaxAPIKeysPerScope {
		t.Fatalf("concurrent results: succeeded=%d limited=%d", succeeded, limited)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not configured")
	}
	if err := migrations.Run(context.Background(), dsn); err != nil {
		t.Fatalf("migrations.Run() error = %v", err)
	}
	repository, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	return repository
}

func identitySeed(suffix, user string) core.IdentitySeed {
	return core.IdentitySeed{
		AccountID: "acc_" + suffix, AccountNamespace: "ns-a-" + suffix,
		Namespace: "ns-u-" + user + "-" + suffix,
	}
}

func testIdentityFingerprint(assertion core.IdentityAssertion) core.IdentityFingerprint {
	domain := sha256.Sum256([]byte("domain:" + assertion.DomainID))
	user := sha256.Sum256([]byte("user:" + assertion.DomainID + "\x00" + assertion.UserID))
	return core.IdentityFingerprint{Domain: domain[:], User: user[:]}
}

func newTestKey(id, name string) core.NewAPIKey {
	digest := sha256.Sum256([]byte(id))
	return core.NewAPIKey{
		ID: id, Name: name, Scope: core.APIKeyScopeDevBridge,
		Mask: "devbridge_test...mask", Digest: digest[:],
	}
}

func keysWithScope(keys []core.APIKey, scope core.APIKeyScope) int {
	count := 0
	for _, key := range keys {
		if key.Scope == scope {
			count++
		}
	}
	return count
}

func keyByID(keys []core.APIKey, id string) core.APIKey {
	for _, key := range keys {
		if key.ID == id {
			return key
		}
	}
	return core.APIKey{}
}
