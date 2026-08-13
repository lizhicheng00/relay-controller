package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"mgmt-service/internal/core"
)

func TestIdentityAndAPIKeyLifecycle(t *testing.T) {
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
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	assertion := core.IdentityAssertion{DomainID: "domain-" + suffix, UserID: "user-a-" + suffix}
	seed := core.IdentitySeed{
		AccountID:        "acc_" + suffix,
		AccountNamespace: "ns-a-" + suffix,
		Namespace:        "ns-u-a-" + suffix,
	}
	defaultKey := newTestKey("key_default_"+suffix, "default", 1)
	identity, err := repository.Provision(ctx, assertion, seed, defaultKey)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	repeated, err := repository.Provision(ctx, assertion, core.IdentitySeed{
		AccountID: "acc-unused", AccountNamespace: "ns-a-unused-" + suffix,
		Namespace: "ns-u-unused-" + suffix,
	}, defaultKey)
	if err != nil || repeated != identity {
		t.Fatalf("Provision(repeated) = %#v, %v; want %#v", repeated, err, identity)
	}
	if found, err := repository.FindIdentity(ctx, defaultKey.Digest); err != nil || found != identity {
		t.Fatalf("FindIdentity(default) = %#v, %v", found, err)
	}

	secondIdentity, err := repository.Provision(ctx, core.IdentityAssertion{
		DomainID: assertion.DomainID, UserID: "user-b-" + suffix,
	}, core.IdentitySeed{
		AccountID: "acc-other", AccountNamespace: "ns-a-other-" + suffix,
		Namespace: "ns-u-b-" + suffix,
	}, newTestKey("key_second_"+suffix, "default", 2))
	if err != nil {
		t.Fatalf("Provision(second user) error = %v", err)
	}
	if secondIdentity.AccountNamespace != identity.AccountNamespace || secondIdentity.Namespace == identity.Namespace {
		t.Fatalf("first = %#v, second = %#v", identity, secondIdentity)
	}

	importedUserID := "user-imported-" + suffix
	importedNamespace := "ns-u-imported-" + suffix
	_, err = repository.db.ExecContext(ctx, `
		INSERT INTO user_identity (account_id, user_id, namespace)
		SELECT id, ?, ? FROM domain_account WHERE domain_id = ?`,
		importedUserID, importedNamespace, assertion.DomainID)
	if err != nil {
		t.Fatalf("preload identity error = %v", err)
	}
	imported, err := repository.Provision(ctx, core.IdentityAssertion{
		DomainID: assertion.DomainID, UserID: importedUserID,
	}, core.IdentitySeed{
		AccountID: "acc-unused-import", AccountNamespace: "ns-a-unused-import-" + suffix,
		Namespace: "ns-u-unused-import-" + suffix,
	}, newTestKey("key_imported_"+suffix, "default", 24))
	if err != nil || imported.Namespace != importedNamespace ||
		imported.AccountNamespace != identity.AccountNamespace {
		t.Fatalf("Provision(imported) = %#v, %v", imported, err)
	}

	additional := make([]core.NewAPIKey, 0, core.MaxAdditionalAPIKeys)
	for index := 1; index <= core.MaxAdditionalAPIKeys; index++ {
		key := newTestKey(fmt.Sprintf("key_%d_%s", index, suffix), fmt.Sprintf("client-%d", index), byte(index+2))
		created, err := repository.CreateAPIKey(ctx, identity.Namespace, key)
		if err != nil || created.Default || created.Name != key.Name {
			t.Fatalf("CreateAPIKey(%d) = %#v, %v", index, created, err)
		}
		additional = append(additional, key)
	}
	if _, err := repository.CreateAPIKey(ctx, identity.Namespace,
		newTestKey("key_overflow_"+suffix, "overflow", 20)); !errors.Is(err, ErrKeyLimit) {
		t.Fatalf("overflow error = %v", err)
	}

	keys, err := repository.ListAPIKeys(ctx, identity.Namespace)
	if err != nil || len(keys) != core.MaxAdditionalAPIKeys+1 || !keys[0].Default {
		t.Fatalf("ListAPIKeys() = %#v, %v", keys, err)
	}
	if _, err := repository.db.ExecContext(ctx, `
		INSERT INTO api_key (id, namespace, slot, name, key_mask, key_hash)
		VALUES (?, ?, 5, 'invalid-slot', 'test...mask', ?)`,
		"key_invalid_"+suffix, identity.Namespace, bytes.Repeat([]byte{99}, 32)); err == nil {
		t.Fatal("database accepted API key slot 5")
	}
	if _, err := repository.CreateAPIKey(ctx, identity.Namespace,
		newTestKey("key_duplicate_"+suffix, additional[0].Name, 21)); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("duplicate name error = %v", err)
	}

	if err := repository.DeleteAPIKey(ctx, identity.Namespace, defaultKey.ID); !errors.Is(err, ErrDefaultKey) {
		t.Fatalf("delete default error = %v", err)
	}
	if err := repository.DeleteAPIKey(ctx, secondIdentity.Namespace, additional[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-namespace delete error = %v", err)
	}
	if err := repository.DeleteAPIKey(ctx, identity.Namespace, additional[0].ID); err != nil {
		t.Fatalf("DeleteAPIKey() error = %v", err)
	}
	if _, err := repository.FindIdentity(ctx, additional[0].Digest); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted key authentication error = %v", err)
	}
	if _, err := repository.CreateAPIKey(ctx, identity.Namespace,
		newTestKey("key_reused_"+suffix, "replacement", 22)); err != nil {
		t.Fatalf("CreateAPIKey(reused slot) error = %v", err)
	}

	rotatedDefault := newTestKey("key_unused_"+suffix, "default", 23)
	if _, err := repository.Provision(ctx, assertion, seed, rotatedDefault); err != nil {
		t.Fatalf("Provision(rotated default) error = %v", err)
	}
	if _, err := repository.FindIdentity(ctx, defaultKey.Digest); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old default authentication error = %v", err)
	}
	if found, err := repository.FindIdentity(ctx, rotatedDefault.Digest); err != nil || found != identity {
		t.Fatalf("rotated identity = %#v, %v", found, err)
	}
}

func TestConcurrentAPIKeyLimit(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not configured")
	}
	repository, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	identity, err := repository.Provision(ctx,
		core.IdentityAssertion{DomainID: "concurrent-domain-" + suffix, UserID: "user-" + suffix},
		core.IdentitySeed{
			AccountID: "acc-c-" + suffix, AccountNamespace: "ns-a-c-" + suffix,
			Namespace: "ns-u-c-" + suffix,
		}, newTestKey("key_default_c_"+suffix, "default", 30))
	if err != nil {
		t.Fatal(err)
	}

	const attempts = 8
	start := make(chan struct{})
	results := make(chan error, attempts)
	var group sync.WaitGroup
	for index := 0; index < attempts; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			_, err := repository.CreateAPIKey(ctx, identity.Namespace, newTestKey(
				fmt.Sprintf("key_c_%d_%s", index, suffix),
				fmt.Sprintf("concurrent-%d", index), byte(40+index)))
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
	if succeeded != core.MaxAdditionalAPIKeys || limited != attempts-core.MaxAdditionalAPIKeys {
		t.Fatalf("concurrent results: succeeded=%d limited=%d", succeeded, limited)
	}
	keys, err := repository.ListAPIKeys(ctx, identity.Namespace)
	if err != nil || len(keys) != core.MaxAdditionalAPIKeys+1 {
		t.Fatalf("ListAPIKeys() count = %d, error = %v", len(keys), err)
	}
}

func newTestKey(id, name string, digestByte byte) core.NewAPIKey {
	return core.NewAPIKey{
		ID: id, Name: name, Mask: "test...mask", Digest: bytes.Repeat([]byte{digestByte}, 32),
	}
}
