package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"mgmt-service/internal/apikey"
	"mgmt-service/internal/domain"
	"mgmt-service/internal/idgen"
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
	store    store.Store
	sessions session.Store
	codec    apikey.Codec
	now      func() time.Time
	logger   *slog.Logger
}

func New(
	repository store.Store,
	sessions session.Store,
	codec apikey.Codec,
	logger *slog.Logger,
) *Service {
	return &Service{
		store: repository, sessions: sessions, codec: codec, now: time.Now, logger: logger,
	}
}

func (s *Service) LoginIAM(
	ctx context.Context,
	iam domain.IAMIdentity,
) (domain.LoginSession, error) {
	if err := validateIAMIdentity(iam); err != nil {
		return domain.LoginSession{}, err
	}
	seed, err := newIdentitySeed(iam.UserName)
	if err != nil {
		return domain.LoginSession{}, internal("failed to generate identity", err)
	}
	identity, err := s.store.ResolveIdentity(ctx, iam, seed)
	if err != nil {
		return domain.LoginSession{}, s.mapStoreError("failed to resolve IAM identity", err)
	}
	token, expiresAt, err := s.sessions.Create(ctx, identity, loginSessionTTL)
	if err != nil {
		return domain.LoginSession{}, internal("failed to create login session", err)
	}
	return domain.LoginSession{LoginToken: token, ExpiresAt: expiresAt, Identity: identity}, nil
}

func (s *Service) IssueLoginAPIKey(
	ctx context.Context,
	loginToken, name, permission string,
) (domain.IssuedAPIKey, error) {
	loginToken = strings.TrimSpace(loginToken)
	if len(loginToken) < 32 || len(loginToken) > 256 || containsControl(loginToken) {
		return domain.IssuedAPIKey{}, unauthorizedTarget("Authorization")
	}
	name, permission, err := validateKeyInput(name, permission)
	if err != nil {
		return domain.IssuedAPIKey{}, err
	}
	identity, err := s.sessions.Consume(ctx, loginToken)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return domain.IssuedAPIKey{}, unauthorizedTarget("Authorization")
		}
		return domain.IssuedAPIKey{}, internal("failed to consume login session", err)
	}
	return s.issueKey(ctx, identity, identity.NamespaceID, name, permission)
}

func (s *Service) Authenticate(ctx context.Context, rawKey string) (domain.AuthContext, error) {
	digest, err := s.codec.Digest(strings.TrimSpace(rawKey))
	if err != nil {
		return domain.AuthContext{}, unauthorizedTarget("X-API-Key")
	}
	credential, err := s.store.FindCredential(ctx, digest)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return domain.AuthContext{}, unauthorizedTarget("X-API-Key")
		}
		return domain.AuthContext{}, internal("failed to authenticate API key", err)
	}
	if credential.Permission != domain.PermissionRead && credential.Permission != domain.PermissionWrite {
		return domain.AuthContext{}, unauthorizedTarget("X-API-Key")
	}
	if err := s.store.TouchAPIKey(ctx, credential.APIKeyID, s.now().UTC()); err != nil {
		s.logger.Debug("failed to update API key usage", "keyId", credential.APIKeyID, "error", err)
	}
	return domain.AuthContext{
		Identity: credential.Identity, Permission: credential.Permission,
	}, nil
}

func (s *Service) ListAPIKeys(
	ctx context.Context,
	identity domain.Identity,
	namespaceID string,
) ([]domain.APIKey, error) {
	if !validIdentifier(namespaceID, 32) {
		return nil, invalid("namespaceId", "namespaceId is invalid")
	}
	keys, err := s.store.ListAPIKeys(ctx, identity.PrincipalID, namespaceID)
	if err != nil {
		return nil, s.mapStoreError("failed to list API keys", err)
	}
	return keys, nil
}

func (s *Service) CreateAPIKey(
	ctx context.Context,
	identity domain.Identity,
	namespaceID, name, permission string,
) (domain.IssuedAPIKey, error) {
	if !validIdentifier(namespaceID, 32) {
		return domain.IssuedAPIKey{}, invalid("namespaceId", "namespaceId is invalid")
	}
	name, permission, err := validateKeyInput(name, permission)
	if err != nil {
		return domain.IssuedAPIKey{}, err
	}
	return s.issueKey(ctx, identity, namespaceID, name, permission)
}

func (s *Service) DeleteAPIKey(
	ctx context.Context,
	identity domain.Identity,
	namespaceID, keyID string,
) error {
	if !validIdentifier(namespaceID, 32) {
		return invalid("namespaceId", "namespaceId is invalid")
	}
	if !validIdentifier(keyID, 32) {
		return invalid("keyId", "keyId is invalid")
	}
	if err := s.store.DeleteAPIKey(ctx, identity.PrincipalID, namespaceID, keyID); err != nil {
		return s.mapStoreError("failed to delete API key", err)
	}
	return nil
}

func (s *Service) CreateNamespace(
	ctx context.Context,
	identity domain.Identity,
	displayName string,
) (domain.Namespace, error) {
	displayName, err := validateDisplayName(displayName)
	if err != nil {
		return domain.Namespace{}, err
	}
	id, err := idgen.New("nsp_")
	if err != nil {
		return domain.Namespace{}, internal("failed to generate namespace", err)
	}
	name, err := idgen.New("ns-u-")
	if err != nil {
		return domain.Namespace{}, internal("failed to generate namespace", err)
	}
	created, err := s.store.CreateNamespace(ctx, domain.NewNamespace{
		ID: id, AccountID: identity.AccountID, PrincipalID: identity.PrincipalID,
		Name: name, DisplayName: displayName,
	})
	if err != nil {
		return domain.Namespace{}, s.mapStoreError("failed to create namespace", err)
	}
	return created, nil
}

func (s *Service) GetNamespace(
	ctx context.Context,
	identity domain.Identity,
	namespaceID string,
) (domain.Namespace, error) {
	if !validIdentifier(namespaceID, 32) {
		return domain.Namespace{}, invalid("namespaceId", "namespaceId is invalid")
	}
	value, err := s.store.GetNamespace(ctx, identity.PrincipalID, namespaceID)
	if err != nil {
		return domain.Namespace{}, s.mapStoreError("failed to get namespace", err)
	}
	return value, nil
}

func (s *Service) ListNamespaces(
	ctx context.Context,
	identity domain.Identity,
) ([]domain.Namespace, error) {
	values, err := s.store.ListNamespaces(ctx, identity.PrincipalID)
	if err != nil {
		return nil, internal("failed to list namespaces", err)
	}
	return values, nil
}

func (s *Service) UpdateNamespace(
	ctx context.Context,
	identity domain.Identity,
	namespaceID, displayName string,
) (domain.Namespace, error) {
	if !validIdentifier(namespaceID, 32) {
		return domain.Namespace{}, invalid("namespaceId", "namespaceId is invalid")
	}
	displayName, err := validateDisplayName(displayName)
	if err != nil {
		return domain.Namespace{}, err
	}
	value, err := s.store.UpdateNamespace(ctx, identity.PrincipalID, namespaceID, displayName)
	if err != nil {
		return domain.Namespace{}, s.mapStoreError("failed to update namespace", err)
	}
	return value, nil
}

func (s *Service) DeleteNamespace(
	ctx context.Context,
	identity domain.Identity,
	namespaceID string,
) error {
	if !validIdentifier(namespaceID, 32) {
		return invalid("namespaceId", "namespaceId is invalid")
	}
	if err := s.store.DeleteNamespace(ctx, identity.PrincipalID, namespaceID); err != nil {
		return s.mapStoreError("failed to delete namespace", err)
	}
	return nil
}

func (s *Service) issueKey(
	ctx context.Context,
	identity domain.Identity,
	namespaceID, name, permission string,
) (domain.IssuedAPIKey, error) {
	id, err := idgen.New("key_")
	if err != nil {
		return domain.IssuedAPIKey{}, internal("failed to generate API key", err)
	}
	value, mask, digest, err := s.codec.Generate()
	if err != nil {
		return domain.IssuedAPIKey{}, internal("failed to generate API key", err)
	}
	created, err := s.store.CreateAPIKey(ctx, identity.PrincipalID, domain.NewAPIKey{
		ID: id, NamespaceID: namespaceID, Name: name, Mask: mask,
		SecretHash: digest, Permission: permission,
	}, maxKeysPerNamespace)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return domain.IssuedAPIKey{}, &Error{
				Kind: KindConflict, Code: "API_KEY_NAME_CONFLICT",
				Message: "an API key with this name already exists in the namespace", Target: "name",
			}
		}
		return domain.IssuedAPIKey{}, s.mapStoreError("failed to issue API key", err)
	}
	return domain.IssuedAPIKey{APIKey: created, Value: value}, nil
}

func (s *Service) mapStoreError(operation string, err error) error {
	switch {
	case errors.Is(err, store.ErrUnauthorized):
		return unauthorizedTarget("X-API-Key")
	case errors.Is(err, store.ErrNotFound):
		return &Error{Kind: KindNotFound, Code: "NOT_FOUND", Message: "resource not found"}
	case errors.Is(err, store.ErrConflict):
		return &Error{Kind: KindConflict, Code: "RESOURCE_CONFLICT", Message: "resource conflicts with existing data"}
	case errors.Is(err, store.ErrKeyLimit):
		return &Error{
			Kind: KindConflict, Code: "API_KEY_LIMIT",
			Message: "a namespace can have at most 5 API keys",
		}
	case errors.Is(err, store.ErrDefaultNamespace):
		return &Error{
			Kind: KindConflict, Code: "DEFAULT_NAMESPACE",
			Message: "the default namespace cannot be deleted", Target: "namespaceId",
		}
	default:
		return internal(operation, err)
	}
}

func newIdentitySeed(userName string) (domain.IdentitySeed, error) {
	accountID, err := idgen.New("acc_")
	if err != nil {
		return domain.IdentitySeed{}, err
	}
	accountNamespace, err := idgen.New("ns-a-")
	if err != nil {
		return domain.IdentitySeed{}, err
	}
	principalID, err := idgen.New("prn_")
	if err != nil {
		return domain.IdentitySeed{}, err
	}
	namespaceID, err := idgen.New("nsp_")
	if err != nil {
		return domain.IdentitySeed{}, err
	}
	namespace, err := idgen.New("ns-u-")
	if err != nil {
		return domain.IdentitySeed{}, err
	}
	displayName := strings.TrimSpace(userName)
	if displayName == "" {
		displayName = defaultNamespaceLabel
	} else if runes := []rune(displayName); len(runes) > 64 {
		displayName = string(runes[:64])
	}
	return domain.IdentitySeed{
		AccountID: accountID, AccountNamespace: accountNamespace, PrincipalID: principalID,
		NamespaceID: namespaceID, Namespace: namespace, DisplayName: displayName,
	}, nil
}

func validateIAMIdentity(identity domain.IAMIdentity) error {
	if !validIdentifier(identity.DomainID, 128) {
		return invalid("X-IAM-Domain-Id", "IAM domain ID is invalid")
	}
	if !validIdentifier(identity.UserID, 128) {
		return invalid("X-IAM-User-Id", "IAM user ID is invalid")
	}
	if !utf8.ValidString(identity.UserName) || utf8.RuneCountInString(identity.UserName) > 128 ||
		containsControl(identity.UserName) {
		return invalid("X-IAM-User-Name", "IAM user name is invalid")
	}
	return nil
}

func validateKeyInput(name, permission string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultKeyName
	}
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > 64 || containsControl(name) {
		return "", "", invalid("name", "name is invalid")
	}
	permission = strings.ToLower(strings.TrimSpace(permission))
	if permission == "" {
		permission = domain.PermissionWrite
	}
	if permission != domain.PermissionRead && permission != domain.PermissionWrite {
		return "", "", invalid("permission", "permission must be read or write")
	}
	return name, permission, nil
}

func validateDisplayName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 64 ||
		containsControl(value) {
		return "", invalid("displayName", "displayName is invalid")
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
