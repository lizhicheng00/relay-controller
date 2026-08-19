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
	IssueDefaultAPIKey(context.Context, core.IdentityAssertion, core.IdentitySeed, core.NewAPIKey) (core.Identity, error)
	EnsureIdentity(context.Context, core.IdentityAssertion, core.IdentitySeed) (core.Identity, error)
	FindIdentity(context.Context, core.IdentityAssertion) (core.Identity, error)
	FindIdentityByAPIKey(context.Context, []byte) (core.Identity, error)
	ListAPIKeys(context.Context, string) ([]core.APIKey, error)
	CreateAPIKey(context.Context, string, core.NewAPIKey) (core.APIKey, error)
	DeleteAPIKey(context.Context, string, string) error
}

type Service struct {
	store repository
}

func New(repository repository) *Service {
	return &Service{store: repository}
}

func (s *Service) IssueDefaultAPIKey(
	ctx context.Context,
	assertion core.IdentityAssertion,
	scope core.APIKeyScope,
) (core.DefaultAPIKeyCredential, error) {
	if err := validateIdentity(assertion); err != nil {
		return core.DefaultAPIKeyCredential{}, err
	}
	if !security.ValidAPIKeyScope(scope) {
		return core.DefaultAPIKeyCredential{}, core.Invalid("scope", "scope must be devbridge or devbox")
	}
	seed := newIdentitySeed()
	value, digest := security.NewAPIKey(scope)
	identity, err := s.store.IssueDefaultAPIKey(ctx, assertion, seed, core.NewAPIKey{
		ID: security.NewID(""), Name: core.DefaultAPIKeyName,
		Scope: scope, Mask: security.MaskAPIKey(value), Digest: digest,
	})
	if err != nil {
		return core.DefaultAPIKeyCredential{}, mapStoreError("issue default API key", "X-User-Id", err)
	}
	return core.DefaultAPIKeyCredential{Identity: identity, Scope: scope, APIKey: value}, nil
}

func (s *Service) CheckAPIKey(ctx context.Context, value string) (core.APIKeyIdentity, error) {
	scope, digest, err := security.ParseAPIKey(strings.TrimSpace(value))
	if err != nil {
		return core.APIKeyIdentity{}, core.Unauthorized("X-API-Key")
	}
	identity, err := s.store.FindIdentityByAPIKey(ctx, digest)
	if err != nil {
		return core.APIKeyIdentity{}, mapStoreError("check API key", "X-API-Key", err)
	}
	return core.APIKeyIdentity{Identity: identity, Scope: scope}, nil
}

func (s *Service) ListAPIKeys(
	ctx context.Context,
	assertion core.IdentityAssertion,
) ([]core.APIKey, error) {
	if err := validateIdentity(assertion); err != nil {
		return nil, err
	}
	identity, err := s.store.FindIdentity(ctx, assertion)
	if errors.Is(err, store.ErrNotFound) {
		return []core.APIKey{}, nil
	}
	if err != nil {
		return nil, core.Internal(fmt.Errorf("resolve identity: %w", err))
	}
	keys, err := s.store.ListAPIKeys(ctx, identity.Namespace)
	if err != nil {
		return nil, core.Internal(fmt.Errorf("list API keys: %w", err))
	}
	return keys, nil
}

func (s *Service) CreateAPIKey(
	ctx context.Context,
	assertion core.IdentityAssertion,
	name string,
	scope core.APIKeyScope,
) (core.IssuedAPIKey, error) {
	name, err := validateAPIKeyName(name)
	if err != nil {
		return core.IssuedAPIKey{}, err
	}
	if !security.ValidAPIKeyScope(scope) {
		return core.IssuedAPIKey{}, core.Invalid("scope", "scope must be devbridge or devbox")
	}
	if err := validateIdentity(assertion); err != nil {
		return core.IssuedAPIKey{}, err
	}
	identity, err := s.store.EnsureIdentity(ctx, assertion, newIdentitySeed())
	if err != nil {
		return core.IssuedAPIKey{}, mapStoreError("ensure identity", "X-User-Id", err)
	}
	value, digest := security.NewAPIKey(scope)
	key, err := s.store.CreateAPIKey(ctx, identity.Namespace, core.NewAPIKey{
		ID: security.NewID(""), Name: name, Scope: scope,
		Mask: security.MaskAPIKey(value), Digest: digest,
	})
	if err != nil {
		return core.IssuedAPIKey{}, mapAPIKeyStoreError("create API key", err)
	}
	return core.IssuedAPIKey{APIKey: key, Value: value}, nil
}

func (s *Service) DeleteAPIKey(
	ctx context.Context,
	assertion core.IdentityAssertion,
	keyID string,
) error {
	if !validKeyID(keyID) {
		return core.Invalid("keyId", "keyId is invalid")
	}
	identity, err := s.resolveIdentity(ctx, assertion)
	if err != nil {
		return err
	}
	if err := s.store.DeleteAPIKey(ctx, identity.Namespace, keyID); err != nil {
		return mapAPIKeyStoreError("delete API key", err)
	}
	return nil
}

func (s *Service) resolveIdentity(
	ctx context.Context,
	assertion core.IdentityAssertion,
) (core.Identity, error) {
	if err := validateIdentity(assertion); err != nil {
		return core.Identity{}, err
	}
	identity, err := s.store.FindIdentity(ctx, assertion)
	if errors.Is(err, store.ErrNotFound) {
		return core.Identity{}, core.IdentityNotFound()
	}
	if err != nil {
		return core.Identity{}, core.Internal(fmt.Errorf("resolve identity: %w", err))
	}
	return identity, nil
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
	if len(value) != 26 {
		return false
	}
	for _, char := range value {
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
		return core.Conflict(core.CodeAPIKeyLimitReached, "apiKeys", "an API key scope can have at most five keys")
	case errors.Is(err, store.ErrNameConflict):
		return core.Conflict(core.CodeAPIKeyNameConflict, "name", "an API key with this name already exists")
	case errors.Is(err, store.ErrDefaultKey):
		return core.Conflict(core.CodeDefaultAPIKey, "keyId", "the default API key cannot be deleted")
	case errors.Is(err, store.ErrNotFound):
		return core.NotFound("keyId", "API key not found")
	case errors.Is(err, store.ErrUnauthorized):
		return core.IdentityNotFound()
	default:
		return core.Internal(fmt.Errorf("%s: %w", operation, err))
	}
}
