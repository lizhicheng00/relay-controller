package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"mgmt-service/internal/core"
	"mgmt-service/internal/security"
	"mgmt-service/internal/store"
)

type repository interface {
	Provision(context.Context, core.IdentityAssertion, core.IdentitySeed, core.NewAPIKey) (core.Identity, error)
	FindIdentity(context.Context, []byte) (core.Identity, error)
	ListAPIKeys(context.Context, string) ([]core.APIKey, error)
	CreateAPIKey(context.Context, string, core.NewAPIKey) (core.APIKey, error)
	DeleteAPIKey(context.Context, string, string) error
}

type Service struct {
	store repository
	keys  security.APIKeys
}

func New(repository repository, keys security.APIKeys) *Service {
	return &Service{store: repository, keys: keys}
}

func (s *Service) ProvisionAPIKey(
	ctx context.Context,
	assertion core.IdentityAssertion,
	keyType core.APIKeyType,
) (core.ProvisionedCredential, error) {
	if err := validateIdentity(assertion); err != nil {
		return core.ProvisionedCredential{}, err
	}
	if !security.ValidAPIKeyType(keyType) {
		return core.ProvisionedCredential{}, core.Invalid("type", "type must be devbridge or devbox")
	}
	seed := newIdentitySeed()
	value, digest := s.keys.DefaultFor(assertion.DomainID, assertion.UserID, keyType)
	identity, err := s.store.Provision(ctx, assertion, seed, core.NewAPIKey{
		ID: security.NewID("key_"), Name: core.DefaultAPIKeyName,
		Type: keyType, Mask: security.MaskAPIKey(value), Digest: digest,
	})
	if err != nil {
		return core.ProvisionedCredential{}, mapStoreError("provision identity", "X-User-Id", err)
	}
	return core.ProvisionedCredential{Identity: identity, Type: keyType, APIKey: value}, nil
}

func (s *Service) Authenticate(ctx context.Context, value string) (core.Identity, error) {
	digest, err := security.DigestAPIKey(strings.TrimSpace(value))
	if err != nil {
		return core.Identity{}, core.Unauthorized("X-API-Key")
	}
	identity, err := s.store.FindIdentity(ctx, digest)
	if err != nil {
		return core.Identity{}, mapStoreError("authenticate API key", "X-API-Key", err)
	}
	return identity, nil
}

func (s *Service) ListAPIKeys(ctx context.Context, identity core.Identity) ([]core.APIKey, error) {
	keys, err := s.store.ListAPIKeys(ctx, identity.Namespace)
	if err != nil {
		return nil, mapStoreError("list API keys", "X-API-Key", err)
	}
	return keys, nil
}

func (s *Service) CreateAPIKey(
	ctx context.Context,
	identity core.Identity,
	name string,
	keyType core.APIKeyType,
) (core.IssuedAPIKey, error) {
	name, err := validateAPIKeyName(name)
	if err != nil {
		return core.IssuedAPIKey{}, err
	}
	if !security.ValidAPIKeyType(keyType) {
		return core.IssuedAPIKey{}, core.Invalid("type", "type must be devbridge or devbox")
	}
	value, digest := s.keys.New(keyType)
	key, err := s.store.CreateAPIKey(ctx, identity.Namespace, core.NewAPIKey{
		ID: security.NewID("key_"), Name: name, Type: keyType,
		Mask: security.MaskAPIKey(value), Digest: digest,
	})
	if err != nil {
		return core.IssuedAPIKey{}, mapAPIKeyStoreError("create API key", err)
	}
	return core.IssuedAPIKey{APIKey: key, Value: value}, nil
}

func (s *Service) DeleteAPIKey(ctx context.Context, identity core.Identity, keyID string) error {
	if !validKeyID(keyID) {
		return core.Invalid("keyId", "keyId is invalid")
	}
	if err := s.store.DeleteAPIKey(ctx, identity.Namespace, keyID); err != nil {
		return mapAPIKeyStoreError("delete API key", err)
	}
	return nil
}

func newIdentitySeed() core.IdentitySeed {
	return core.IdentitySeed{
		AccountID: security.NewID("acc_"), AccountNamespace: security.NewID("ns-a-"),
		Namespace: security.NewID("ns-u-"),
	}
}

func validateIdentity(assertion core.IdentityAssertion) error {
	if !validIdentifier(assertion.DomainID) {
		return core.Invalid("X-Domain-Id", "domain ID is invalid")
	}
	if !validIdentifier(assertion.UserID) {
		return core.Invalid("X-User-Id", "user ID is invalid")
	}
	return nil
}

func validateAPIKeyName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 64 {
		return "", core.Invalid("name", "name must contain 1 to 64 characters")
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return "", core.Invalid("name", "name contains an invalid character")
		}
	}
	if strings.EqualFold(value, core.DefaultAPIKeyName) {
		return "", core.Invalid("name", "name is reserved")
	}
	return value, nil
}

func validIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || index > 0 && strings.ContainsRune("._:-", char) {
			continue
		}
		return false
	}
	return true
}

func validKeyID(value string) bool {
	if len(value) != 30 || !strings.HasPrefix(value, "key_") {
		return false
	}
	for _, char := range value[4:] {
		if char < 'a' || char > 'z' {
			if char < '2' || char > '7' {
				return false
			}
		}
	}
	return true
}

func mapStoreError(operation, target string, err error) error {
	if errors.Is(err, store.ErrUnauthorized) || errors.Is(err, store.ErrNotFound) {
		return core.Unauthorized(target)
	}
	return core.Internal(fmt.Errorf("%s: %w", operation, err))
}

func mapAPIKeyStoreError(operation string, err error) error {
	switch {
	case errors.Is(err, store.ErrKeyLimit):
		return core.Conflict(core.CodeAPIKeyLimitReached, "apiKeys", "an API key type can have at most five keys")
	case errors.Is(err, store.ErrNameConflict):
		return core.Conflict(core.CodeAPIKeyNameConflict, "name", "an API key with this name already exists")
	case errors.Is(err, store.ErrDefaultKey):
		return core.Conflict(core.CodeDefaultAPIKey, "keyId", "the default API key cannot be deleted")
	case errors.Is(err, store.ErrNotFound):
		return core.NotFound("keyId", "API key not found")
	case errors.Is(err, store.ErrUnauthorized):
		return core.Unauthorized("X-API-Key")
	default:
		return core.Internal(fmt.Errorf("%s: %w", operation, err))
	}
}
