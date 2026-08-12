package redisstore

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

	"mgmt-service/internal/domain"
	"mgmt-service/internal/session"
)

const tokenBytes = 32

type Store struct {
	client *redis.Client
}

type payload struct {
	PrincipalID string `json:"principalId"`
	NamespaceID string `json:"namespaceId"`
}

func Open(address, password string) *Store {
	return &Store{client: redis.NewClient(&redis.Options{
		Addr: address, Password: password,
	})}
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
	identity domain.Identity,
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

func (s *Store) Consume(ctx context.Context, token string) (domain.Identity, error) {
	value, err := s.client.GetDel(ctx, redisKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return domain.Identity{}, session.ErrNotFound
	}
	if err != nil {
		return domain.Identity{}, fmt.Errorf("consume login session: %w", err)
	}
	var stored payload
	if err := json.Unmarshal(value, &stored); err != nil {
		return domain.Identity{}, fmt.Errorf("decode login session: %w", err)
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

func payloadFromIdentity(identity domain.Identity) payload {
	return payload{
		PrincipalID: identity.PrincipalID,
		NamespaceID: identity.NamespaceID,
	}
}

func (p payload) identity() domain.Identity {
	return domain.Identity{
		PrincipalID: p.PrincipalID,
		NamespaceID: p.NamespaceID,
	}
}
