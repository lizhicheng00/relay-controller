package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"mgmt-service/internal/core"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database DSN: %w", err)
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	cfg.Params["charset"] = "utf8mb4"
	cfg.Params["time_zone"] = "'+00:00'"

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	pingContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingContext); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Provision(
	ctx context.Context,
	assertion core.IdentityAssertion,
	seed core.IdentitySeed,
	apiKeyHash []byte,
) (core.Identity, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Identity{}, fmt.Errorf("begin provision transaction: %w", err)
	}
	defer rollback(tx)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO domain_account (id, domain_id, account_namespace)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE id = id`,
		seed.AccountID, assertion.DomainID, seed.AccountNamespace)
	if err != nil {
		return core.Identity{}, fmt.Errorf("create domain account: %w", err)
	}

	var identity core.Identity
	var accountID, accountStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT id, domain_id, account_namespace, status
		FROM domain_account
		WHERE domain_id = ?
		FOR UPDATE`, assertion.DomainID).Scan(
		&accountID, &identity.DomainID, &identity.AccountNamespace, &accountStatus)
	if err != nil {
		return core.Identity{}, mapQueryError("load domain account", err)
	}
	if accountStatus != "active" {
		return core.Identity{}, ErrUnauthorized
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_identity (account_id, user_id, namespace, api_key_hash)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE account_id = account_id`,
		accountID, assertion.UserID, seed.Namespace, apiKeyHash)
	if err != nil {
		return core.Identity{}, fmt.Errorf("create user identity: %w", err)
	}

	var storedHash []byte
	var userStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT user_id, namespace, api_key_hash, status
		FROM user_identity
		WHERE account_id = ? AND user_id = ?
		FOR UPDATE`, accountID, assertion.UserID).Scan(
		&identity.UserID, &identity.Namespace, &storedHash, &userStatus)
	if err != nil {
		return core.Identity{}, mapQueryError("load user identity", err)
	}
	if userStatus != "active" {
		return core.Identity{}, ErrUnauthorized
	}
	if !bytes.Equal(storedHash, apiKeyHash) {
		if _, err := tx.ExecContext(ctx, `
			UPDATE user_identity SET api_key_hash = ?
			WHERE account_id = ? AND user_id = ?`,
			apiKeyHash, accountID, assertion.UserID); err != nil {
			return core.Identity{}, fmt.Errorf("rotate API key: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return core.Identity{}, fmt.Errorf("commit provision transaction: %w", err)
	}
	return identity, nil
}

func (s *Store) FindIdentity(ctx context.Context, apiKeyHash []byte) (core.Identity, error) {
	var identity core.Identity
	err := s.db.QueryRowContext(ctx, `
		SELECT a.domain_id, u.user_id, a.account_namespace, u.namespace
		FROM user_identity u
		JOIN domain_account a ON a.id = u.account_id
		WHERE u.api_key_hash = ?
		  AND a.status = 'active'
		  AND u.status = 'active'`, apiKeyHash).Scan(
		&identity.DomainID,
		&identity.UserID,
		&identity.AccountNamespace,
		&identity.Namespace,
	)
	if err != nil {
		return core.Identity{}, mapQueryError("find identity", err)
	}
	return identity, nil
}

func mapQueryError(operation string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func rollback(tx *sql.Tx) { _ = tx.Rollback() }
