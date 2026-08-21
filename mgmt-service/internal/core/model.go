package core

import "time"

const (
	CLILoginAPIKeyName = "CLI login"
	MaxAPIKeysPerScope = 20
)

type APIKeyScope string

const (
	APIKeyScopeDevBridge APIKeyScope = "devbridge"
	APIKeyScopeDevBox    APIKeyScope = "devbox"
)

type APIKeySource string

const (
	APIKeySourceCLILogin    APIKeySource = "cli_login"
	APIKeySourceUserCreated APIKeySource = "user_created"
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

type APIKeyIdentity struct {
	Identity
	Scope APIKeyScope `json:"scope"`
}

type CLILoginCredential struct {
	Identity
	Scope  APIKeyScope `json:"scope"`
	APIKey string      `json:"apiKey"`
}

type NewAPIKey struct {
	ID     string
	Name   string
	Scope  APIKeyScope
	Mask   string
	Digest []byte
	Source APIKeySource
}

type APIKey struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Scope      APIKeyScope  `json:"scope"`
	Mask       string       `json:"mask"`
	Source     APIKeySource `json:"source"`
	CreatedAt  time.Time    `json:"createdAt"`
	LastUsedAt *time.Time   `json:"lastUsedAt"`
}

type IssuedAPIKey struct {
	APIKey
	Value string `json:"apiKey"`
}
