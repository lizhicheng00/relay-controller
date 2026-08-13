package core

import "time"

const (
	DefaultAPIKeyName    = "default"
	MaxAdditionalAPIKeys = 4
)

type APIKeyScenario string

const (
	APIKeyScenarioDevBridge APIKeyScenario = "devbridge"
	APIKeyScenarioDevBox    APIKeyScenario = "devbox"
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

type ProvisionedCredential struct {
	Identity
	APIKey string `json:"apiKey"`
}

type NewAPIKey struct {
	ID       string
	Name     string
	Scenario APIKeyScenario
	Mask     string
	Digest   []byte
}

type APIKey struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Scenario  APIKeyScenario `json:"scenario"`
	Mask      string         `json:"mask"`
	Default   bool           `json:"isDefault"`
	CreatedAt time.Time      `json:"createdAt"`
}

type IssuedAPIKey struct {
	APIKey
	Value string `json:"apiKey"`
}
