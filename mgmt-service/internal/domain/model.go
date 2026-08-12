package domain

import "time"

const (
	PermissionRead  = "read"
	PermissionWrite = "write"
)

type IAMIdentity struct {
	DomainID string
	UserID   string
	UserName string
}

type IdentitySeed struct {
	AccountID        string
	AccountNamespace string
	PrincipalID      string
	NamespaceID      string
	Namespace        string
	DisplayName      string
}

type Identity struct {
	AccountID        string `json:"-"`
	AccountNamespace string `json:"accountNamespace"`
	PrincipalID      string `json:"principalId"`
	NamespaceID      string `json:"namespaceId"`
	Namespace        string `json:"namespace"`
	IAMUserName      string `json:"iamUserName,omitempty"`
}

type AuthContext struct {
	Identity
	Permission string `json:"permission"`
}

type LoginSession struct {
	LoginToken string    `json:"loginToken"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Identity   Identity  `json:"identity"`
}

type NewAPIKey struct {
	ID          string
	NamespaceID string
	Name        string
	Mask        string
	SecretHash  []byte
	Permission  string
}

type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Mask       string     `json:"mask"`
	Permission string     `json:"permission"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type IssuedAPIKey struct {
	APIKey
	Value string `json:"value"`
}

type Credential struct {
	Identity
	APIKeyID   string
	Permission string
}

type NewNamespace struct {
	ID          string
	AccountID   string
	PrincipalID string
	Name        string
	DisplayName string
}

type Namespace struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Default     bool      `json:"default"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
