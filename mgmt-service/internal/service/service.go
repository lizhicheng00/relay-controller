package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"mgmt-service/internal/core"
	"mgmt-service/internal/security"
	"mgmt-service/internal/store"
)

type repository interface {
	EnsureIdentity(context.Context, core.IdentityFingerprint, core.IdentitySeed) (core.Identity, error)
	FindIdentity(context.Context, core.IdentityFingerprint) (core.Identity, error)
	FindIdentityByAPIKey(context.Context, []byte) (core.Identity, error)
	ListAPIKeys(context.Context, string) ([]core.APIKey, error)
	CreateAPIKey(context.Context, string, core.NewAPIKey) (core.APIKey, error)
	DeleteAPIKey(context.Context, string, string) error
}

type fingerprinter interface {
	Fingerprint(string, ...string) []byte
}

type Service struct {
	store         repository
	fingerprinter fingerprinter
}

var (
	identityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	keyIDPattern    = regexp.MustCompile(`^[a-z2-7]{26}$`)
)

func New(repository repository, identityFingerprinter fingerprinter) *Service {
	return &Service{store: repository, fingerprinter: identityFingerprinter}
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

func (s *Service) ResolveIdentity(
	ctx context.Context,
	assertion core.IdentityAssertion,
) (core.Identity, error) {
	return s.ensureIdentity(ctx, assertion)
}

func (s *Service) ListAPIKeys(
	ctx context.Context,
	assertion core.IdentityAssertion,
) ([]core.APIKey, error) {
	if err := validateIdentity(assertion); err != nil {
		return nil, err
	}
	identity, err := s.store.FindIdentity(ctx, s.fingerprint(assertion))
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
	identity, err := s.ensureIdentity(ctx, assertion)
	if err != nil {
		return core.IssuedAPIKey{}, err
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

func (s *Service) ensureIdentity(
	ctx context.Context,
	assertion core.IdentityAssertion,
) (core.Identity, error) {
	if err := validateIdentity(assertion); err != nil {
		return core.Identity{}, err
	}
	identity, err := s.store.EnsureIdentity(ctx, s.fingerprint(assertion), newIdentitySeed())
	if err != nil {
		return core.Identity{}, mapStoreError("ensure identity", "X-User-Id", err)
	}
	return identity, nil
}

func (s *Service) DeleteAPIKey(
	ctx context.Context,
	assertion core.IdentityAssertion,
	keyID string,
) error {
	if !keyIDPattern.MatchString(keyID) {
		return core.Invalid("keyId", "keyId is invalid")
	}
	identity, err := s.findIdentity(ctx, assertion)
	if err != nil {
		return err
	}
	if err := s.store.DeleteAPIKey(ctx, identity.Namespace, keyID); err != nil {
		return mapAPIKeyStoreError("delete API key", err)
	}
	return nil
}

func (s *Service) findIdentity(
	ctx context.Context,
	assertion core.IdentityAssertion,
) (core.Identity, error) {
	if err := validateIdentity(assertion); err != nil {
		return core.Identity{}, err
	}
	identity, err := s.store.FindIdentity(ctx, s.fingerprint(assertion))
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
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", core.Invalid("name", "name contains an invalid character")
	}
	return value, nil
}

func validIdentifier(value string) bool {
	return identityPattern.MatchString(value)
}

func (s *Service) fingerprint(assertion core.IdentityAssertion) core.IdentityFingerprint {
	return core.IdentityFingerprint{
		Domain: s.fingerprinter.Fingerprint("domain", assertion.DomainID),
		User:   s.fingerprinter.Fingerprint("user", assertion.DomainID, assertion.UserID),
	}
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
		return core.Conflict(core.CodeAPIKeyLimitReached, "apiKeys", "API key limit exceeded for this scope")
	case errors.Is(err, store.ErrNotFound):
		return core.NotFound("keyId", "API key not found")
	case errors.Is(err, store.ErrUnauthorized):
		return core.IdentityNotFound()
	default:
		return core.Internal(fmt.Errorf("%s: %w", operation, err))
	}
}
