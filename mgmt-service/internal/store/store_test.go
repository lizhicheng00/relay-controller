package store

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"mgmt-service/internal/core"
)

func TestProvisionAndAuthenticationLifecycle(t *testing.T) {
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
	firstAssertion := core.IdentityAssertion{DomainID: "domain-" + suffix, UserID: "user-a-" + suffix}
	firstSeed := core.IdentitySeed{
		AccountID:        "acc_" + suffix,
		AccountNamespace: "ns-a-" + suffix,
		Namespace:        "ns-u-a-" + suffix,
	}
	firstHash := bytes.Repeat([]byte{1}, 32)
	first, err := repository.Provision(ctx, firstAssertion, firstSeed, firstHash)
	if err != nil {
		t.Fatalf("Provision(first) error = %v", err)
	}

	repeated, err := repository.Provision(ctx, firstAssertion, core.IdentitySeed{
		AccountID: "acc-unused", AccountNamespace: "ns-a-unused-" + suffix,
		Namespace: "ns-u-unused-" + suffix,
	}, firstHash)
	if err != nil || repeated != first {
		t.Fatalf("Provision(repeated) = %#v, %v; want %#v", repeated, err, first)
	}

	second, err := repository.Provision(ctx, core.IdentityAssertion{
		DomainID: firstAssertion.DomainID, UserID: "user-b-" + suffix,
	}, core.IdentitySeed{
		AccountID: "acc-other", AccountNamespace: "ns-a-other-" + suffix,
		Namespace: "ns-u-b-" + suffix,
	}, bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatalf("Provision(second) error = %v", err)
	}
	if second.AccountNamespace != first.AccountNamespace || second.Namespace == first.Namespace {
		t.Fatalf("first = %#v, second = %#v", first, second)
	}

	found, err := repository.FindIdentity(ctx, firstHash)
	if err != nil || found != first {
		t.Fatalf("FindIdentity() = %#v, %v; want %#v", found, err, first)
	}

	rotatedHash := bytes.Repeat([]byte{3}, 32)
	if _, err := repository.Provision(ctx, firstAssertion, firstSeed, rotatedHash); err != nil {
		t.Fatalf("Provision(rotated) error = %v", err)
	}
	if _, err := repository.FindIdentity(ctx, firstHash); err != ErrNotFound {
		t.Fatalf("old API key error = %v", err)
	}
	if found, err := repository.FindIdentity(ctx, rotatedHash); err != nil || found != first {
		t.Fatalf("rotated identity = %#v, %v", found, err)
	}
}
