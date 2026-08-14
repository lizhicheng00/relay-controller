package core

import "time"

const (
	DefaultAPIKeyName           = "default"
	MaxAPIKeysPerType           = 5
	MaxAdditionalAPIKeysPerType = MaxAPIKeysPerType - 1
)

type APIKeyType string

const (
	APIKeyTypeDevBridge APIKeyType = "devbridge"
	APIKeyTypeDevBox    APIKeyType = "devbox"
)

type IdentityAssertion struct {
	DomainID string
	UserID   string
}

type IdentitySeed struct {
	AccountID        string
	AccountNamespace string
	Namespace        string
}

type Identity struct {
	DomainID         string `json:"domainId"`
	UserID           string `json:"userId"`
	AccountNamespace string `json:"accountNamespace"`
	Namespace        string `json:"namespace"`
}

type DefaultAPIKeyCredential struct {
	Identity
	Type   APIKeyType `json:"type"`
	APIKey string     `json:"apiKey"`
}

type NewAPIKey struct {
	ID     string
	Name   string
	Type   APIKeyType
	Mask   string
	Digest []byte
}

type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       APIKeyType `json:"type"`
	Mask       string     `json:"mask"`
	Default    bool       `json:"isDefault"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
}

type IssuedAPIKey struct {
	APIKey
	Value string `json:"apiKey"`
}
