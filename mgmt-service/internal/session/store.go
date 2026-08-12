package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"mgmt-service/internal/core"
)

const tokenBytes = 32

var ErrNotFound = errors.New("login session not found")

type Store struct {
	client *redis.Client
}

type payload struct {
	PrincipalID string `json:"principalId"`
	NamespaceID string `json:"namespaceId"`
}

func Open(ctx context.Context, address, password string) (*Store, error) {
	store := &Store{client: redis.NewClient(&redis.Options{
		Addr: address, Password: password,
	})}
	pingContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := store.Ping(pingContext); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Ping(ctx context.Context) error {
	if err := s.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping Redis: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.client.Close()
}

func (s *Store) Create(
	ctx context.Context,
	identity core.Identity,
	ttl time.Duration,
) (string, time.Time, error) {
	value, err := json.Marshal(payloadFromIdentity(identity))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("encode login session: %w", err)
	}
	for range 3 {
		token, err := randomToken()
		if err != nil {
			return "", time.Time{}, err
		}
		stored, err := s.client.SetNX(ctx, redisKey(token), value, ttl).Result()
		if err != nil {
			return "", time.Time{}, fmt.Errorf("store login session: %w", err)
		}
		if stored {
			return token, time.Now().UTC().Add(ttl), nil
		}
	}
	return "", time.Time{}, errors.New("generate unique login session")
}

func (s *Store) Consume(ctx context.Context, token string) (core.Identity, error) {
	value, err := s.client.GetDel(ctx, redisKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return core.Identity{}, ErrNotFound
	}
	if err != nil {
		return core.Identity{}, fmt.Errorf("consume login session: %w", err)
	}
	var stored payload
	if err := json.Unmarshal(value, &stored); err != nil {
		return core.Identity{}, fmt.Errorf("decode login session: %w", err)
	}
	return stored.identity(), nil
}

func randomToken() (string, error) {
	value := make([]byte, tokenBytes)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate login token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func redisKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return "mgmt:login:" + hex.EncodeToString(digest[:])
}

func payloadFromIdentity(identity core.Identity) payload {
	return payload{
		PrincipalID: identity.PrincipalID,
		NamespaceID: identity.NamespaceID,
	}
}

func (p payload) identity() core.Identity {
	return core.Identity{
		PrincipalID: p.PrincipalID,
		NamespaceID: p.NamespaceID,
	}
}
