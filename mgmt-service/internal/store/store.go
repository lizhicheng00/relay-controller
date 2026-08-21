package store

import (
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
	ErrKeyLimit     = errors.New("API key limit exceeded")
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

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) EnsureIdentity(
	ctx context.Context,
	assertion core.IdentityAssertion,
	seed core.IdentitySeed,
) (core.Identity, error) {
	var identity core.Identity
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var err error
		identity, err = ensureIdentity(ctx, tx, assertion, seed)
		return err
	})
	return identity, err
}

func ensureIdentity(
	ctx context.Context,
	tx *sql.Tx,
	assertion core.IdentityAssertion,
	seed core.IdentitySeed,
) (core.Identity, error) {
	_, err := tx.ExecContext(ctx, `
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
		INSERT INTO user_identity (account_id, user_id, namespace)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE account_id = account_id`,
		accountID, assertion.UserID, seed.Namespace)
	if err != nil {
		return core.Identity{}, fmt.Errorf("create user identity: %w", err)
	}

	var userStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT user_id, namespace, status
		FROM user_identity
		WHERE account_id = ? AND user_id = ?
		FOR UPDATE`, accountID, assertion.UserID).Scan(
		&identity.UserID, &identity.Namespace, &userStatus)
	if err != nil {
		return core.Identity{}, mapQueryError("load user identity", err)
	}
	if userStatus != "active" {
		return core.Identity{}, ErrUnauthorized
	}

	return identity, nil
}

func (s *Store) FindIdentity(
	ctx context.Context,
	assertion core.IdentityAssertion,
) (core.Identity, error) {
	var identity core.Identity
	err := s.db.QueryRowContext(ctx, `
		SELECT a.domain_id, u.user_id, a.account_namespace, u.namespace
		FROM domain_account a
		JOIN user_identity u ON u.account_id = a.id
		WHERE a.domain_id = ?
		  AND u.user_id = ?
		  AND a.status = 'active'
		  AND u.status = 'active'`, assertion.DomainID, assertion.UserID).Scan(
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

func (s *Store) FindIdentityByAPIKey(ctx context.Context, apiKeyHash []byte) (core.Identity, error) {
	var identity core.Identity
	var keyID string
	err := s.db.QueryRowContext(ctx, `
		SELECT a.domain_id, u.user_id, a.account_namespace, u.namespace, k.id
		FROM api_key k
		JOIN user_identity u ON u.namespace = k.namespace
		JOIN domain_account a ON a.id = u.account_id
		WHERE k.key_hash = ?
		  AND a.status = 'active'
		  AND u.status = 'active'`, apiKeyHash).Scan(
		&identity.DomainID,
		&identity.UserID,
		&identity.AccountNamespace,
		&identity.Namespace,
		&keyID,
	)
	if err != nil {
		return core.Identity{}, mapQueryError("find identity", err)
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE api_key
		SET last_used_at = UTC_TIMESTAMP(6)
		WHERE id = ?
		  AND (last_used_at IS NULL OR last_used_at < UTC_TIMESTAMP(6) - INTERVAL 1 MINUTE)`,
		keyID)
	if err != nil {
		return core.Identity{}, fmt.Errorf("update API key last use: %w", err)
	}
	return identity, nil
}

func (s *Store) ListAPIKeys(ctx context.Context, namespace string) ([]core.APIKey, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, key_scope, key_mask, source, created_at, last_used_at
		FROM api_key
		WHERE namespace = ?
		ORDER BY key_scope, created_at DESC, id`, namespace)
	if err != nil {
		return nil, fmt.Errorf("list API keys: %w", err)
	}
	defer rows.Close()

	keys := make([]core.APIKey, 0, core.MaxAPIKeysPerScope*2)
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate API keys: %w", err)
	}
	return keys, nil
}

func (s *Store) CreateAPIKey(
	ctx context.Context,
	namespace string,
	key core.NewAPIKey,
) (core.APIKey, error) {
	var created core.APIKey
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if err := lockIdentity(ctx, tx, namespace); err != nil {
			return err
		}
		var count int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM api_key
			WHERE namespace = ? AND key_scope = ?`, namespace, key.Scope).Scan(&count); err != nil {
			return fmt.Errorf("count API keys: %w", err)
		}
		if count >= core.MaxAPIKeysPerScope {
			return ErrKeyLimit
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO api_key (id, namespace, name, key_scope, source, key_mask, key_hash)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			key.ID, namespace, key.Name, key.Scope, key.Source, key.Mask, key.Digest)
		if err != nil {
			return fmt.Errorf("create API key: %w", err)
		}
		created, err = queryAPIKey(ctx, tx, namespace, key.ID)
		if err != nil {
			return err
		}
		return nil
	})
	return created, err
}

func (s *Store) DeleteAPIKey(ctx context.Context, namespace, keyID string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := lockIdentity(ctx, tx, namespace); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM api_key WHERE namespace = ? AND id = ?`, namespace, keyID)
		if err != nil {
			return fmt.Errorf("delete API key: %w", err)
		}
		deleted, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read deleted API key count: %w", err)
		}
		if deleted == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func lockIdentity(ctx context.Context, tx *sql.Tx, namespace string) error {
	var status string
	err := tx.QueryRowContext(ctx, `
		SELECT status FROM user_identity WHERE namespace = ? FOR UPDATE`, namespace).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnauthorized
	}
	if err != nil {
		return mapQueryError("lock user identity", err)
	}
	if status != "active" {
		return ErrUnauthorized
	}
	return nil
}

func queryAPIKey(ctx context.Context, tx *sql.Tx, namespace, keyID string) (core.APIKey, error) {
	return scanAPIKey(tx.QueryRowContext(ctx, `
		SELECT id, name, key_scope, key_mask, source, created_at, last_used_at
		FROM api_key WHERE namespace = ? AND id = ?`, namespace, keyID))
}

type scanner interface {
	Scan(...any) error
}

func scanAPIKey(row scanner) (core.APIKey, error) {
	var key core.APIKey
	if err := row.Scan(
		&key.ID, &key.Name, &key.Scope, &key.Mask, &key.Source, &key.CreatedAt, &key.LastUsedAt,
	); err != nil {
		return core.APIKey{}, mapQueryError("load API key", err)
	}
	return key, nil
}

func mapQueryError(operation string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func (s *Store) inTx(ctx context.Context, operation func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := operation(tx); err != nil {
		return err
	}
	return tx.Commit()
}
