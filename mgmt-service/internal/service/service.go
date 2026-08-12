package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"mgmt-service/internal/core"
	"mgmt-service/internal/security"
	"mgmt-service/internal/session"
	"mgmt-service/internal/store"
)

const (
	defaultKeyName        = "default"
	loginSessionTTL       = 5 * time.Minute
	maxKeysPerNamespace   = 5
	defaultNamespaceLabel = "Default"
)

type Service struct {
	store    repository
	sessions sessionStore
	keys     security.APIKeyCodec
	now      func() time.Time
	logger   *slog.Logger
}

type repository interface {
	ResolveIdentity(context.Context, core.IAMIdentity, core.IdentitySeed) (core.Identity, error)
	CreateAPIKey(context.Context, string, core.NewAPIKey, int) (core.APIKey, error)
	DeleteAPIKey(context.Context, string, string, string) error
	ListAPIKeys(context.Context, string, string) ([]core.APIKey, error)
	FindCredential(context.Context, []byte) (core.Credential, error)
	TouchAPIKey(context.Context, string, time.Time) error
	CreateNamespace(context.Context, core.NewNamespace) (core.Namespace, error)
	GetNamespace(context.Context, string, string) (core.Namespace, error)
	ListNamespaces(context.Context, string) ([]core.Namespace, error)
	UpdateNamespace(context.Context, string, string, string) (core.Namespace, error)
	DeleteNamespace(context.Context, string, string) error
}

type sessionStore interface {
	Create(context.Context, core.Identity, time.Duration) (string, time.Time, error)
	Consume(context.Context, string) (core.Identity, error)
}

func New(
	repository repository,
	sessions sessionStore,
	keys security.APIKeyCodec,
	logger *slog.Logger,
) *Service {
	return &Service{
		store: repository, sessions: sessions, keys: keys, now: time.Now, logger: logger,
	}
}

func (s *Service) LoginIAM(
	ctx context.Context,
	iam core.IAMIdentity,
) (core.LoginSession, error) {
	if err := validateIAMIdentity(iam); err != nil {
		return core.LoginSession{}, err
	}
	seed, err := newIdentitySeed(iam.UserName)
	if err != nil {
		return core.LoginSession{}, core.Internal("failed to generate identity", err)
	}
	identity, err := s.store.ResolveIdentity(ctx, iam, seed)
	if err != nil {
		return core.LoginSession{}, s.mapStoreError("failed to resolve IAM identity", err)
	}
	token, expiresAt, err := s.sessions.Create(ctx, identity, loginSessionTTL)
	if err != nil {
		return core.LoginSession{}, core.Internal("failed to create login session", err)
	}
	return core.LoginSession{LoginToken: token, ExpiresAt: expiresAt, Identity: identity}, nil
}

func (s *Service) IssueLoginAPIKey(
	ctx context.Context,
	loginToken, name, permission string,
) (core.IssuedAPIKey, error) {
	loginToken = strings.TrimSpace(loginToken)
	if len(loginToken) < 32 || len(loginToken) > 256 || containsControl(loginToken) {
		return core.IssuedAPIKey{}, core.Unauthorized("Authorization")
	}
	name, permission, err := validateKeyInput(name, permission)
	if err != nil {
		return core.IssuedAPIKey{}, err
	}
	identity, err := s.sessions.Consume(ctx, loginToken)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return core.IssuedAPIKey{}, core.Unauthorized("Authorization")
		}
		return core.IssuedAPIKey{}, core.Internal("failed to consume login session", err)
	}
	return s.issueKey(ctx, identity, identity.NamespaceID, name, permission)
}

func (s *Service) Authenticate(ctx context.Context, rawKey string) (core.AuthContext, error) {
	digest, err := s.keys.Digest(strings.TrimSpace(rawKey))
	if err != nil {
		return core.AuthContext{}, core.Unauthorized("X-API-Key")
	}
	credential, err := s.store.FindCredential(ctx, digest)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return core.AuthContext{}, core.Unauthorized("X-API-Key")
		}
		return core.AuthContext{}, core.Internal("failed to authenticate API key", err)
	}
	if credential.Permission != core.PermissionRead && credential.Permission != core.PermissionWrite {
		return core.AuthContext{}, core.Unauthorized("X-API-Key")
	}
	if err := s.store.TouchAPIKey(ctx, credential.APIKeyID, s.now().UTC()); err != nil {
		s.logger.Debug("failed to update API key usage", "keyId", credential.APIKeyID, "error", err)
	}
	return core.AuthContext{
		Identity: credential.Identity, Permission: credential.Permission,
	}, nil
}

func (s *Service) ListAPIKeys(
	ctx context.Context,
	identity core.Identity,
	namespaceID string,
) ([]core.APIKey, error) {
	if !validIdentifier(namespaceID, 32) {
		return nil, core.Invalid("namespaceId", "namespaceId is invalid")
	}
	keys, err := s.store.ListAPIKeys(ctx, identity.PrincipalID, namespaceID)
	if err != nil {
		return nil, s.mapStoreError("failed to list API keys", err)
	}
	return keys, nil
}

func (s *Service) CreateAPIKey(
	ctx context.Context,
	identity core.Identity,
	namespaceID, name, permission string,
) (core.IssuedAPIKey, error) {
	if !validIdentifier(namespaceID, 32) {
		return core.IssuedAPIKey{}, core.Invalid("namespaceId", "namespaceId is invalid")
	}
	name, permission, err := validateKeyInput(name, permission)
	if err != nil {
		return core.IssuedAPIKey{}, err
	}
	return s.issueKey(ctx, identity, namespaceID, name, permission)
}

func (s *Service) DeleteAPIKey(
	ctx context.Context,
	identity core.Identity,
	namespaceID, keyID string,
) error {
	if !validIdentifier(namespaceID, 32) {
		return core.Invalid("namespaceId", "namespaceId is invalid")
	}
	if !validIdentifier(keyID, 32) {
		return core.Invalid("keyId", "keyId is invalid")
	}
	if err := s.store.DeleteAPIKey(ctx, identity.PrincipalID, namespaceID, keyID); err != nil {
		return s.mapStoreError("failed to delete API key", err)
	}
	return nil
}

func (s *Service) CreateNamespace(
	ctx context.Context,
	identity core.Identity,
	displayName string,
) (core.Namespace, error) {
	displayName, err := validateDisplayName(displayName)
	if err != nil {
		return core.Namespace{}, err
	}
	id, err := security.NewID("nsp_")
	if err != nil {
		return core.Namespace{}, core.Internal("failed to generate namespace", err)
	}
	name, err := security.NewID("ns-u-")
	if err != nil {
		return core.Namespace{}, core.Internal("failed to generate namespace", err)
	}
	created, err := s.store.CreateNamespace(ctx, core.NewNamespace{
		ID: id, AccountID: identity.AccountID, PrincipalID: identity.PrincipalID,
		Name: name, DisplayName: displayName,
	})
	if err != nil {
		return core.Namespace{}, s.mapStoreError("failed to create namespace", err)
	}
	return created, nil
}

func (s *Service) GetNamespace(
	ctx context.Context,
	identity core.Identity,
	namespaceID string,
) (core.Namespace, error) {
	if !validIdentifier(namespaceID, 32) {
		return core.Namespace{}, core.Invalid("namespaceId", "namespaceId is invalid")
	}
	value, err := s.store.GetNamespace(ctx, identity.PrincipalID, namespaceID)
	if err != nil {
		return core.Namespace{}, s.mapStoreError("failed to get namespace", err)
	}
	return value, nil
}

func (s *Service) ListNamespaces(
	ctx context.Context,
	identity core.Identity,
) ([]core.Namespace, error) {
	values, err := s.store.ListNamespaces(ctx, identity.PrincipalID)
	if err != nil {
		return nil, core.Internal("failed to list namespaces", err)
	}
	return values, nil
}

func (s *Service) UpdateNamespace(
	ctx context.Context,
	identity core.Identity,
	namespaceID, displayName string,
) (core.Namespace, error) {
	if !validIdentifier(namespaceID, 32) {
		return core.Namespace{}, core.Invalid("namespaceId", "namespaceId is invalid")
	}
	displayName, err := validateDisplayName(displayName)
	if err != nil {
		return core.Namespace{}, err
	}
	value, err := s.store.UpdateNamespace(ctx, identity.PrincipalID, namespaceID, displayName)
	if err != nil {
		return core.Namespace{}, s.mapStoreError("failed to update namespace", err)
	}
	return value, nil
}

func (s *Service) DeleteNamespace(
	ctx context.Context,
	identity core.Identity,
	namespaceID string,
) error {
	if !validIdentifier(namespaceID, 32) {
		return core.Invalid("namespaceId", "namespaceId is invalid")
	}
	if err := s.store.DeleteNamespace(ctx, identity.PrincipalID, namespaceID); err != nil {
		return s.mapStoreError("failed to delete namespace", err)
	}
	return nil
}

func (s *Service) issueKey(
	ctx context.Context,
	identity core.Identity,
	namespaceID, name, permission string,
) (core.IssuedAPIKey, error) {
	id, err := security.NewID("key_")
	if err != nil {
		return core.IssuedAPIKey{}, core.Internal("failed to generate API key", err)
	}
	value, mask, digest, err := s.keys.Generate()
	if err != nil {
		return core.IssuedAPIKey{}, core.Internal("failed to generate API key", err)
	}
	created, err := s.store.CreateAPIKey(ctx, identity.PrincipalID, core.NewAPIKey{
		ID: id, NamespaceID: namespaceID, Name: name, Mask: mask,
		SecretHash: digest, Permission: permission,
	}, maxKeysPerNamespace)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return core.IssuedAPIKey{}, core.Conflict("API_KEY_NAME_CONFLICT",
				"an API key with this name already exists in the namespace", "name")
		}
		return core.IssuedAPIKey{}, s.mapStoreError("failed to issue API key", err)
	}
	return core.IssuedAPIKey{APIKey: created, Value: value}, nil
}

func (s *Service) mapStoreError(operation string, err error) error {
	switch {
	case errors.Is(err, store.ErrUnauthorized):
		return core.Unauthorized("X-API-Key")
	case errors.Is(err, store.ErrNotFound):
		return core.NotFound()
	case errors.Is(err, store.ErrConflict):
		return core.Conflict("RESOURCE_CONFLICT", "resource conflicts with existing data", "")
	case errors.Is(err, store.ErrKeyLimit):
		return core.Conflict("API_KEY_LIMIT", "a namespace can have at most 5 API keys", "")
	case errors.Is(err, store.ErrDefaultNamespace):
		return core.Conflict("DEFAULT_NAMESPACE",
			"the default namespace cannot be deleted", "namespaceId")
	default:
		return core.Internal(operation, err)
	}
}

func newIdentitySeed(userName string) (core.IdentitySeed, error) {
	accountID, err := security.NewID("acc_")
	if err != nil {
		return core.IdentitySeed{}, err
	}
	accountNamespace, err := security.NewID("ns-a-")
	if err != nil {
		return core.IdentitySeed{}, err
	}
	principalID, err := security.NewID("prn_")
	if err != nil {
		return core.IdentitySeed{}, err
	}
	namespaceID, err := security.NewID("nsp_")
	if err != nil {
		return core.IdentitySeed{}, err
	}
	namespace, err := security.NewID("ns-u-")
	if err != nil {
		return core.IdentitySeed{}, err
	}
	displayName := strings.TrimSpace(userName)
	if displayName == "" {
		displayName = defaultNamespaceLabel
	} else if runes := []rune(displayName); len(runes) > 64 {
		displayName = string(runes[:64])
	}
	return core.IdentitySeed{
		AccountID: accountID, AccountNamespace: accountNamespace, PrincipalID: principalID,
		NamespaceID: namespaceID, Namespace: namespace, DisplayName: displayName,
	}, nil
}

func validateIAMIdentity(identity core.IAMIdentity) error {
	if !validIdentifier(identity.DomainID, 128) {
		return core.Invalid("X-IAM-Domain-Id", "IAM domain ID is invalid")
	}
	if !validIdentifier(identity.UserID, 128) {
		return core.Invalid("X-IAM-User-Id", "IAM user ID is invalid")
	}
	if !utf8.ValidString(identity.UserName) || utf8.RuneCountInString(identity.UserName) > 128 ||
		containsControl(identity.UserName) {
		return core.Invalid("X-IAM-User-Name", "IAM user name is invalid")
	}
	return nil
}

func validateKeyInput(name, permission string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultKeyName
	}
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > 64 || containsControl(name) {
		return "", "", core.Invalid("name", "name is invalid")
	}
	permission = strings.ToLower(strings.TrimSpace(permission))
	if permission == "" {
		permission = core.PermissionWrite
	}
	if permission != core.PermissionRead && permission != core.PermissionWrite {
		return "", "", core.Invalid("permission", "permission must be read or write")
	}
	return name, permission, nil
}

func validateDisplayName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 64 ||
		containsControl(value) {
		return "", core.Invalid("displayName", "displayName is invalid")
	}
	return value, nil
}

func validIdentifier(value string, maxLength int) bool {
	if len(value) == 0 || len(value) > maxLength {
		return false
	}
	for index, char := range value {
		if isAlphaNumeric(char) || index > 0 && strings.ContainsRune("._:-", char) {
			continue
		}
		return false
	}
	return true
}

func isAlphaNumeric(char rune) bool {
	return char >= 'a' && char <= 'z' ||
		char >= 'A' && char <= 'Z' ||
		char >= '0' && char <= '9'
}

func containsControl(value string) bool {
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return true
		}
	}
	return false
}
