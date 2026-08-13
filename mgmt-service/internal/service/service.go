package service

import (
	"context"
	"errors"
	"strings"

	"mgmt-service/internal/core"
	"mgmt-service/internal/security"
	"mgmt-service/internal/store"
)

type repository interface {
	Provision(context.Context, core.IdentityAssertion, core.IdentitySeed, []byte) (core.Identity, error)
	FindIdentity(context.Context, []byte) (core.Identity, error)
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
) (core.ProvisionedCredential, error) {
	if err := validateIdentity(assertion); err != nil {
		return core.ProvisionedCredential{}, err
	}
	seed, err := newIdentitySeed()
	if err != nil {
		return core.ProvisionedCredential{}, core.Internal("generate identity", err)
	}
	value, digest := s.keys.For(assertion.DomainID, assertion.UserID)
	identity, err := s.store.Provision(ctx, assertion, seed, digest)
	if err != nil {
		return core.ProvisionedCredential{}, mapStoreError("provision identity", "X-User-Id", err)
	}
	return core.ProvisionedCredential{Identity: identity, APIKey: value}, nil
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

func newIdentitySeed() (core.IdentitySeed, error) {
	accountID, err := security.NewID("acc_")
	if err != nil {
		return core.IdentitySeed{}, err
	}
	accountNamespace, err := security.NewID("ns-a-")
	if err != nil {
		return core.IdentitySeed{}, err
	}
	namespace, err := security.NewID("ns-u-")
	if err != nil {
		return core.IdentitySeed{}, err
	}
	return core.IdentitySeed{
		AccountID: accountID, AccountNamespace: accountNamespace, Namespace: namespace,
	}, nil
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

func mapStoreError(operation, target string, err error) error {
	if errors.Is(err, store.ErrUnauthorized) || errors.Is(err, store.ErrNotFound) {
		return core.Unauthorized(target)
	}
	return core.Internal(operation, err)
}
