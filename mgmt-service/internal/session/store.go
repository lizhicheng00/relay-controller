package session

import (
	"context"
	"errors"
	"time"

	"mgmt-service/internal/domain"
)

var ErrNotFound = errors.New("login session not found")

type Store interface {
	Ping(context.Context) error
	Close() error
	Create(context.Context, domain.Identity, time.Duration) (string, time.Time, error)
	Consume(context.Context, string) (domain.Identity, error)
}
