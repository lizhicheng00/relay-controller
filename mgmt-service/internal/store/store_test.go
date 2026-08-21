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
	defer repository.Close()

	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	assertion := core.IdentityAssertion{DomainID: "domain-" + suffix, UserID: "user-a-" + suffix}
	seed := core.IdentitySeed{
		AccountID:        "acc_" + suffix,
		AccountNamespace: "ns-a-" + suffix,
		Namespace:        "ns-u-a-" + suffix,
	}
	defaultKey := newTestKey("key_default_"+suffix, "default")
	identity, err := repository.IssueDefaultAPIKey(ctx, assertion, seed, defaultKey)
	if err != nil {
		t.Fatalf("IssueDefaultAPIKey() error = %v", err)
	}

	_, err = repository.IssueDefaultAPIKey(ctx, assertion, core.IdentitySeed{
		AccountID: "acc-unused", AccountNamespace: "ns-a-unused-" + suffix,
		Namespace: "ns-u-unused-" + suffix,
	}, defaultKey)
	if !errors.Is(err, ErrDefaultKeyExists) {
		t.Fatalf("IssueDefaultAPIKey(repeated) error = %v", err)
	}
	if found, err := repository.FindIdentity(ctx, assertion); err != nil || found != identity {
		t.Fatalf("FindIdentity() = %#v, %v", found, err)
	}
	keysBeforeUse, err := repository.ListAPIKeys(ctx, identity.Namespace)
	if err != nil || keyByID(keysBeforeUse, defaultKey.ID).LastUsedAt != nil {
		t.Fatalf("unused default metadata = %#v, %v", keyByID(keysBeforeUse, defaultKey.ID), err)
	}
	if found, err := repository.FindIdentityByAPIKey(ctx, defaultKey.Digest); err != nil || found != identity {
		t.Fatalf("FindIdentityByAPIKey(default) = %#v, %v", found, err)
	}

	secondIdentity, err := repository.IssueDefaultAPIKey(ctx, core.IdentityAssertion{
		DomainID: assertion.DomainID, UserID: "user-b-" + suffix,
	}, core.IdentitySeed{
		AccountID: "acc-other", AccountNamespace: "ns-a-other-" + suffix,
		Namespace: "ns-u-b-" + suffix,
	}, newTestKey("key_second_"+suffix, "default"))
	if err != nil {
		t.Fatalf("IssueDefaultAPIKey(second user) error = %v", err)
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
	imported, err := repository.IssueDefaultAPIKey(ctx, core.IdentityAssertion{
		DomainID: assertion.DomainID, UserID: importedUserID,
	}, core.IdentitySeed{
		AccountID: "acc-unused-import", AccountNamespace: "ns-a-unused-import-" + suffix,
		Namespace: "ns-u-unused-import-" + suffix,
	}, newTestKey("key_imported_"+suffix, "default"))
	if err != nil || imported.Namespace != importedNamespace ||
		imported.AccountNamespace != identity.AccountNamespace {
		t.Fatalf("IssueDefaultAPIKey(imported) = %#v, %v", imported, err)
	}

	devboxDefault := newTestKey("key_devbox_default_"+suffix, "default")
	devboxDefault.Scope = core.APIKeyScopeDevBox
	devboxDefault.Mask = "devbox_test...mask"
	if _, err := repository.IssueDefaultAPIKey(ctx, assertion, seed, devboxDefault); err != nil {
		t.Fatalf("IssueDefaultAPIKey(devbox default) error = %v", err)
	}

	additional := make([]core.NewAPIKey, 0, core.MaxAdditionalAPIKeysPerScope)
	for index := 1; index <= core.MaxAdditionalAPIKeysPerScope; index++ {
		key := newTestKey(fmt.Sprintf("key_%d_%s", index, suffix), fmt.Sprintf("client-%d", index))
		created, err := repository.CreateAPIKey(ctx, identity.Namespace, key)
		if err != nil || created.Default || created.Name != key.Name {
			t.Fatalf("CreateAPIKey(%d) = %#v, %v", index, created, err)
		}
		additional = append(additional, key)
	}
	if _, err := repository.CreateAPIKey(ctx, identity.Namespace,
		newTestKey("key_overflow_"+suffix, "overflow")); !errors.Is(err, ErrKeyLimit) {
		t.Fatalf("overflow error = %v", err)
	}
	for index := 1; index <= core.MaxAdditionalAPIKeysPerScope; index++ {
		key := newTestKey(fmt.Sprintf("key_box_%d_%s", index, suffix), fmt.Sprintf("client-%d", index))
		key.Scope = core.APIKeyScopeDevBox
		key.Mask = "devbox_test...mask"
		if _, err := repository.CreateAPIKey(ctx, identity.Namespace, key); err != nil {
			t.Fatalf("CreateAPIKey(devbox %d) error = %v", index, err)
		}
	}
	devboxOverflow := newTestKey("key_box_overflow_"+suffix, "overflow")
	devboxOverflow.Scope = core.APIKeyScopeDevBox
	if _, err := repository.CreateAPIKey(ctx, identity.Namespace, devboxOverflow); !errors.Is(err, ErrKeyLimit) {
		t.Fatalf("devbox overflow error = %v", err)
	}

	keys, err := repository.ListAPIKeys(ctx, identity.Namespace)
	if err != nil || len(keys) != core.MaxAPIKeysPerScope*2 || countDefaultKeys(keys) != 2 ||
		keysWithScope(keys, core.APIKeyScopeDevBridge) != core.MaxAPIKeysPerScope ||
		keysWithScope(keys, core.APIKeyScopeDevBox) != core.MaxAPIKeysPerScope {
		t.Fatalf("ListAPIKeys() = %#v, %v", keys, err)
	}
	if keyByID(keys, defaultKey.ID).LastUsedAt == nil {
		t.Fatal("authenticated key has no last-used time")
	}
	if _, err := repository.db.ExecContext(ctx, `
		INSERT INTO api_key (id, namespace, slot, name, key_scope, key_mask, key_hash)
		VALUES (?, ?, 5, 'invalid-slot', 'devbridge', 'devbridge_test...mask', ?)`,
		"key_invalid_"+suffix, identity.Namespace, bytes.Repeat([]byte{99}, 32)); err == nil {
		t.Fatal("database accepted API key slot 5")
	}
	if _, err := repository.db.ExecContext(ctx, `
		INSERT INTO api_key (id, namespace, slot, name, key_scope, key_mask, key_hash)
		VALUES (?, ?, 1, 'invalid-type', 'unknown', 'unknown_test...mask', ?)`,
		"key_bad_scope_"+suffix, secondIdentity.Namespace,
		bytes.Repeat([]byte{98}, 32)); err == nil {
		t.Fatal("database accepted an unknown API key scope")
	}
	if _, err := repository.CreateAPIKey(ctx, identity.Namespace,
		newTestKey("key_duplicate_"+suffix, additional[0].Name)); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("duplicate name error = %v", err)
	}

	if err := repository.DeleteAPIKey(ctx, identity.Namespace, defaultKey.ID); err != nil {
		t.Fatalf("delete default error = %v", err)
	}
	if _, err := repository.FindIdentityByAPIKey(ctx, defaultKey.Digest); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted default authentication error = %v", err)
	}
	if err := repository.DeleteAPIKey(ctx, secondIdentity.Namespace, additional[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-namespace delete error = %v", err)
	}
	if err := repository.DeleteAPIKey(ctx, identity.Namespace, additional[0].ID); err != nil {
		t.Fatalf("DeleteAPIKey() error = %v", err)
	}
	if _, err := repository.FindIdentityByAPIKey(ctx, additional[0].Digest); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted key authentication error = %v", err)
	}
	if _, err := repository.CreateAPIKey(ctx, identity.Namespace,
		newTestKey("key_reused_"+suffix, "replacement")); err != nil {
		t.Fatalf("CreateAPIKey(reused slot) error = %v", err)
	}

	replacementDefault := newTestKey("key_unused_"+suffix, "default")
	if _, err := repository.IssueDefaultAPIKey(ctx, assertion, seed, replacementDefault); err != nil {
		t.Fatalf("IssueDefaultAPIKey(replacement default) error = %v", err)
	}
	if _, err := repository.FindIdentityByAPIKey(ctx, defaultKey.Digest); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old default authentication error = %v", err)
	}
	keys, err = repository.ListAPIKeys(ctx, identity.Namespace)
	if err != nil || keyByID(keys, defaultKey.ID).ID != "" ||
		keyByID(keys, replacementDefault.ID).ID == "" {
		t.Fatalf("replacement default metadata = %#v, %v", keys, err)
	}
	if found, err := repository.FindIdentityByAPIKey(ctx, devboxDefault.Digest); err != nil || found != identity {
		t.Fatalf("other scope default = %#v, %v", found, err)
	}
	if found, err := repository.FindIdentityByAPIKey(ctx, replacementDefault.Digest); err != nil || found != identity {
		t.Fatalf("replacement identity = %#v, %v", found, err)
	}
}

func TestConcurrentAPIKeyLimit(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not configured")
	}
	if err := migrations.Run(context.Background(), dsn); err != nil {
		t.Fatalf("migrations.Run() error = %v", err)
	}
	repository, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	identity, err := repository.IssueDefaultAPIKey(ctx,
		core.IdentityAssertion{DomainID: "concurrent-domain-" + suffix, UserID: "user-" + suffix},
		core.IdentitySeed{
			AccountID: "acc-c-" + suffix, AccountNamespace: "ns-a-c-" + suffix,
			Namespace: "ns-u-c-" + suffix,
		}, newTestKey("key_default_c_"+suffix, "default"))
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
				fmt.Sprintf("concurrent-%d", index)))
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
	if succeeded != core.MaxAdditionalAPIKeysPerScope || limited != attempts-core.MaxAdditionalAPIKeysPerScope {
		t.Fatalf("concurrent results: succeeded=%d limited=%d", succeeded, limited)
	}
	keys, err := repository.ListAPIKeys(ctx, identity.Namespace)
	if err != nil || len(keys) != core.MaxAPIKeysPerScope {
		t.Fatalf("ListAPIKeys() count = %d, error = %v", len(keys), err)
	}
}

func newTestKey(id, name string) core.NewAPIKey {
	digest := sha256.Sum256([]byte(id))
	return core.NewAPIKey{
		ID: id, Name: name, Scope: core.APIKeyScopeDevBridge,
		Mask: "devbridge_test...mask", Digest: digest[:],
	}
}

func countDefaultKeys(keys []core.APIKey) int {
	count := 0
	for _, key := range keys {
		if key.Default {
			count++
		}
	}
	return count
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
