package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"mgmt-service/internal/config"
	"mgmt-service/internal/secret"
)

const namespaceSource = `
[
  {"namespace": "namespace", "user_domain_id": "domain-id"}
]
`

const userSource = `
[
  {"user_id": "user-id", "customer_id": "domain-id"}
]
`

type identity struct {
	domainID  string
	userID    string
	namespace string
}

type namespaceRecord struct {
	Namespace string `json:"namespace"`
	DomainID  string `json:"user_domain_id"`
}

type userRecord struct {
	UserID     string `json:"user_id"`
	CustomerID string `json:"customer_id"`
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return convert(namespaceSource, userSource, os.Stdout, cfg.Secrets)
}

func convert(namespacesJSON, usersJSON string, output io.Writer, codec *secret.Codec) error {
	identities, err := joinIdentities(namespacesJSON, usersJSON)
	if err != nil {
		return err
	}
	if len(identities) == 0 {
		return fmt.Errorf("input contains no identities")
	}

	_, _ = fmt.Fprintln(output, "INSERT INTO legacy_identity")
	_, _ = fmt.Fprintln(output, "    (identity_type, account_mapping_key, member_mapping_key, namespace)")
	_, _ = fmt.Fprintln(output, "VALUES")
	for index, item := range identities {
		accountKey := codec.Fingerprint("domain", item.domainID)
		memberKey := codec.Fingerprint("user", item.domainID, item.userID)
		separator := ","
		if index == len(identities)-1 {
			separator = ";"
		}
		_, _ = fmt.Fprintf(output, "    ('%s', UNHEX('%s'), UNHEX('%s'), '%s')%s\n",
			"main", hex.EncodeToString(accountKey), hex.EncodeToString(memberKey),
			strings.ReplaceAll(item.namespace, "'", "''"), separator)
	}
	return nil
}

func joinIdentities(namespacesJSON, usersJSON string) ([]identity, error) {
	var namespaces []namespaceRecord
	if err := json.Unmarshal([]byte(namespacesJSON), &namespaces); err != nil {
		return nil, fmt.Errorf("parse namespace JSON: %w", err)
	}
	var users []userRecord
	if err := json.Unmarshal([]byte(usersJSON), &users); err != nil {
		return nil, fmt.Errorf("parse user JSON: %w", err)
	}

	usersByCustomer := make(map[string]string, len(users))
	for _, user := range users {
		if existing, found := usersByCustomer[user.CustomerID]; found && existing != user.UserID {
			return nil, fmt.Errorf("customer %q maps to multiple users", user.CustomerID)
		}
		usersByCustomer[user.CustomerID] = user.UserID
	}

	identities := make([]identity, 0, len(namespaces))
	for _, namespace := range namespaces {
		userID, found := usersByCustomer[namespace.DomainID]
		if !found {
			return nil, fmt.Errorf("domain %q has no matching user", namespace.DomainID)
		}
		identities = append(identities, identity{
			domainID: namespace.DomainID, userID: userID, namespace: namespace.Namespace,
		})
	}
	return identities, nil
}
