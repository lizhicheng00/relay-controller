package session

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"mgmt-service/internal/core"
)

func TestLoginSessionIsOpaqueAndSingleUse(t *testing.T) {
	address := os.Getenv("TEST_REDIS_ADDRESS")
	if address == "" {
		t.Skip("TEST_REDIS_ADDRESS is not configured")
	}
	store, err := Open(context.Background(), address, "")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	identity := core.Identity{
		AccountID: "acc-test", PrincipalID: "prn-test",
		NamespaceID: "nsp-test", Namespace: "ns-test",
	}
	token, expiresAt, err := store.Create(ctx, identity, 5*time.Minute)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(token) < 32 || time.Until(expiresAt) <= 0 {
		t.Fatalf("Create() token length = %d, expiresAt = %v", len(token), expiresAt)
	}
	ttl, err := store.client.TTL(ctx, redisKey(token)).Result()
	if err != nil || ttl <= 0 || ttl > 5*time.Minute {
		t.Fatalf("login session TTL = %v, error = %v", ttl, err)
	}
	keys, err := store.client.Keys(ctx, "mgmt:login:*").Result()
	if err != nil {
		t.Fatalf("list Redis keys: %v", err)
	}
	for _, key := range keys {
		if strings.Contains(key, token) {
			t.Fatalf("Redis key exposes login token: %q", key)
		}
	}

	consumed, err := store.Consume(ctx, token)
	if err != nil || consumed.NamespaceID != identity.NamespaceID ||
		consumed.PrincipalID != identity.PrincipalID {
		t.Fatalf("Consume() = %#v, %v", consumed, err)
	}
	if _, err := store.Consume(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Consume() error = %v", err)
	}
}
