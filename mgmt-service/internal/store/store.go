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
	ErrKeyLimit     = errors.New("API key limit reached")
	ErrNameConflict = errors.New("API key name already exists")
	ErrDefaultKey   = errors.New("default API key cannot be deleted")
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

func (s *Store) Provision(
	ctx context.Context,
	assertion core.IdentityAssertion,
	seed core.IdentitySeed,
	defaultKey core.NewAPIKey,
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

	var existingDefaultID string
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM api_key
		WHERE namespace = ? AND key_type = ? AND slot = 0
		FOR UPDATE`, identity.Namespace, defaultKey.Type).Scan(&existingDefaultID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, `
			INSERT INTO api_key (id, namespace, slot, name, key_type, key_mask, key_hash)
			VALUES (?, ?, 0, ?, ?, ?, ?)`,
			defaultKey.ID, identity.Namespace, core.DefaultAPIKeyName,
			defaultKey.Type, defaultKey.Mask, defaultKey.Digest)
	case err == nil:
		_, err = tx.ExecContext(ctx, `
			UPDATE api_key SET name = ?, key_mask = ?, key_hash = ? WHERE id = ?`,
			core.DefaultAPIKeyName, defaultKey.Mask, defaultKey.Digest, existingDefaultID)
	}
	if err != nil {
		return core.Identity{}, fmt.Errorf("store default API key: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return core.Identity{}, fmt.Errorf("commit provision transaction: %w", err)
	}
	return identity, nil
}

func (s *Store) FindIdentity(ctx context.Context, apiKeyHash []byte) (core.Identity, error) {
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
		SELECT k.id, k.name, k.key_type, k.key_mask, k.slot, k.created_at, k.last_used_at
		FROM api_key k
		JOIN user_identity u ON u.namespace = k.namespace
		WHERE k.namespace = ? AND u.status = 'active'
		ORDER BY k.key_type, k.slot`, namespace)
	if err != nil {
		return nil, fmt.Errorf("list API keys: %w", err)
	}
	defer rows.Close()

	keys := make([]core.APIKey, 0, core.MaxAPIKeysPerType*2)
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.APIKey{}, fmt.Errorf("begin API key transaction: %w", err)
	}
	defer rollback(tx)

	if err := lockIdentity(ctx, tx, namespace); err != nil {
		return core.APIKey{}, err
	}
	slot, err := availableSlot(ctx, tx, namespace, key.Type, key.Name)
	if err != nil {
		return core.APIKey{}, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO api_key (id, namespace, slot, name, key_type, key_mask, key_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key.ID, namespace, slot, key.Name, key.Type, key.Mask, key.Digest)
	if err != nil {
		var mysqlError *mysqldriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return core.APIKey{}, ErrNameConflict
		}
		return core.APIKey{}, fmt.Errorf("create API key: %w", err)
	}
	created, err := queryAPIKey(ctx, tx, namespace, key.ID)
	if err != nil {
		return core.APIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return core.APIKey{}, fmt.Errorf("commit API key transaction: %w", err)
	}
	return created, nil
}

func (s *Store) DeleteAPIKey(ctx context.Context, namespace, keyID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin API key deletion: %w", err)
	}
	defer rollback(tx)

	if err := lockIdentity(ctx, tx, namespace); err != nil {
		return err
	}
	var slot int
	err = tx.QueryRowContext(ctx, `
		SELECT slot FROM api_key WHERE namespace = ? AND id = ? FOR UPDATE`,
		namespace, keyID).Scan(&slot)
	if err != nil {
		return mapQueryError("load API key", err)
	}
	if slot == 0 {
		return ErrDefaultKey
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM api_key WHERE namespace = ? AND id = ?`,
		namespace, keyID); err != nil {
		return fmt.Errorf("delete API key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit API key deletion: %w", err)
	}
	return nil
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

func availableSlot(
	ctx context.Context,
	tx *sql.Tx,
	namespace string,
	keyType core.APIKeyType,
	name string,
) (int, error) {
	var occupied [core.MaxAPIKeysPerType]bool
	rows, err := tx.QueryContext(ctx, `
		SELECT slot, name FROM api_key
		WHERE namespace = ? AND key_type = ?
		FOR UPDATE`, namespace, keyType)
	if err != nil {
		return 0, fmt.Errorf("list API key slots: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var slot int
		var existingName string
		if err := rows.Scan(&slot, &existingName); err != nil {
			return 0, fmt.Errorf("scan API key slot: %w", err)
		}
		if existingName == name {
			return 0, ErrNameConflict
		}
		occupied[slot] = true
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate API key slots: %w", err)
	}
	for slot := 1; slot < core.MaxAPIKeysPerType; slot++ {
		if !occupied[slot] {
			return slot, nil
		}
	}
	return 0, ErrKeyLimit
}

func queryAPIKey(ctx context.Context, tx *sql.Tx, namespace, keyID string) (core.APIKey, error) {
	return scanAPIKey(tx.QueryRowContext(ctx, `
		SELECT id, name, key_type, key_mask, slot, created_at, last_used_at
		FROM api_key WHERE namespace = ? AND id = ?`, namespace, keyID))
}

type scanner interface {
	Scan(...any) error
}

func scanAPIKey(row scanner) (core.APIKey, error) {
	var key core.APIKey
	var slot int
	if err := row.Scan(
		&key.ID, &key.Name, &key.Type, &key.Mask, &slot, &key.CreatedAt, &key.LastUsedAt,
	); err != nil {
		return core.APIKey{}, mapQueryError("load API key", err)
	}
	key.Default = slot == 0
	return key, nil
}

func mapQueryError(operation string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func rollback(tx *sql.Tx) { _ = tx.Rollback() }
