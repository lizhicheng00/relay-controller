package store

import (
	"context"
	"errors"
	"time"

	"mgmt-service/internal/domain"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("conflict")
	ErrKeyLimit         = errors.New("API key limit reached")
	ErrDefaultNamespace = errors.New("default namespace cannot be deleted")
	ErrUnauthorized     = errors.New("unauthorized")
)

type Store interface {
	Ping(context.Context) error
	Close() error
	ResolveIdentity(context.Context, domain.IAMIdentity, domain.IdentitySeed) (domain.Identity, error)

	CreateAPIKey(context.Context, string, domain.NewAPIKey, int) (domain.APIKey, error)
	DeleteAPIKey(context.Context, string, string, string) error
	ListAPIKeys(context.Context, string, string) ([]domain.APIKey, error)
	FindCredential(context.Context, []byte) (domain.Credential, error)
	TouchAPIKey(context.Context, string, time.Time) error

	CreateNamespace(context.Context, domain.NewNamespace) (domain.Namespace, error)
	GetNamespace(context.Context, string, string) (domain.Namespace, error)
	ListNamespaces(context.Context, string) ([]domain.Namespace, error)
	UpdateNamespace(context.Context, string, string, string) (domain.Namespace, error)
	DeleteNamespace(context.Context, string, string) error
}
