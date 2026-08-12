package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"mgmt-service/internal/domain"
	"mgmt-service/internal/store"
)

type Store struct {
	db *sql.DB
}

func Open(dsn string) (*Store, error) {
	db, err := openDB(dsn, false)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func OpenMigrationDB(dsn string) (*sql.DB, error) {
	return openDB(dsn, true)
}

func openDB(dsn string, multiStatements bool) (*sql.DB, error) {
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database DSN: %w", err)
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.MultiStatements = multiStatements
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
	return db, nil
}

func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) ResolveIdentity(
	ctx context.Context,
	iam domain.IAMIdentity,
	seed domain.IdentitySeed,
) (domain.Identity, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Identity{}, fmt.Errorf("begin identity transaction: %w", err)
	}
	defer rollback(tx)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO iam_account (id, iam_domain_id, account_namespace)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE id = id`,
		seed.AccountID, iam.DomainID, seed.AccountNamespace)
	if err != nil {
		return domain.Identity{}, mapWriteError("upsert IAM account", err)
	}

	var identity domain.Identity
	var accountStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT id, account_namespace, status
		FROM iam_account
		WHERE iam_domain_id = ?
		FOR UPDATE`, iam.DomainID).Scan(
		&identity.AccountID, &identity.AccountNamespace, &accountStatus)
	if err != nil {
		return domain.Identity{}, mapQueryError("load IAM account", err)
	}
	if accountStatus != "active" {
		return domain.Identity{}, store.ErrUnauthorized
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO iam_principal (id, account_id, iam_user_id, iam_user_name)
		VALUES (?, ?, ?, NULLIF(?, ''))
		ON DUPLICATE KEY UPDATE id = id`,
		seed.PrincipalID, identity.AccountID, iam.UserID, iam.UserName)
	if err != nil {
		return domain.Identity{}, mapWriteError("upsert IAM principal", err)
	}

	var principalStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(iam_user_name, ''), status
		FROM iam_principal
		WHERE account_id = ? AND iam_user_id = ?
		FOR UPDATE`, identity.AccountID, iam.UserID).Scan(
		&identity.PrincipalID, &identity.IAMUserName, &principalStatus)
	if err != nil {
		return domain.Identity{}, mapQueryError("load IAM principal", err)
	}
	if principalStatus != "active" {
		return domain.Identity{}, store.ErrUnauthorized
	}
	if iam.UserName != "" && iam.UserName != identity.IAMUserName {
		_, err = tx.ExecContext(ctx, `UPDATE iam_principal SET iam_user_name = ? WHERE id = ?`,
			iam.UserName, identity.PrincipalID)
		if err != nil {
			return domain.Identity{}, fmt.Errorf("update IAM user name: %w", err)
		}
		identity.IAMUserName = iam.UserName
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO namespace (
			id, account_id, owner_principal_id, name, display_name, is_default
		) VALUES (?, ?, ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE id = id`,
		seed.NamespaceID, identity.AccountID, identity.PrincipalID,
		seed.Namespace, seed.DisplayName)
	if err != nil {
		return domain.Identity{}, mapWriteError("upsert default namespace", err)
	}

	var namespaceStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT id, name, status
		FROM namespace
		WHERE owner_principal_id = ? AND is_default = 1
		FOR UPDATE`, identity.PrincipalID).Scan(
		&identity.NamespaceID, &identity.Namespace, &namespaceStatus)
	if err != nil {
		return domain.Identity{}, mapQueryError("load default namespace", err)
	}
	if namespaceStatus != "active" {
		return domain.Identity{}, store.ErrUnauthorized
	}

	if err := tx.Commit(); err != nil {
		return domain.Identity{}, fmt.Errorf("commit identity transaction: %w", err)
	}
	return identity, nil
}

func (s *Store) CreateAPIKey(
	ctx context.Context,
	principalID string,
	key domain.NewAPIKey,
	maxKeys int,
) (domain.APIKey, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.APIKey{}, fmt.Errorf("begin API key transaction: %w", err)
	}
	defer rollback(tx)

	if err := lockNamespace(ctx, tx, principalID, key.NamespaceID); err != nil {
		return domain.APIKey{}, err
	}
	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM api_key WHERE namespace_id = ?`, key.NamespaceID,
	).Scan(&count); err != nil {
		return domain.APIKey{}, fmt.Errorf("count API keys: %w", err)
	}
	if count >= maxKeys {
		return domain.APIKey{}, store.ErrKeyLimit
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO api_key (id, namespace_id, name, key_mask, secret_hash, permission)
		VALUES (?, ?, ?, ?, ?, ?)`,
		key.ID, key.NamespaceID, key.Name, key.Mask, key.SecretHash, key.Permission)
	if err != nil {
		return domain.APIKey{}, mapWriteError("insert API key", err)
	}
	created, err := queryAPIKey(ctx, tx, key.ID)
	if err != nil {
		return domain.APIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKey{}, fmt.Errorf("commit API key transaction: %w", err)
	}
	return created, nil
}

func (s *Store) DeleteAPIKey(
	ctx context.Context,
	principalID, namespaceID, keyID string,
) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE k FROM api_key k
		JOIN namespace n ON n.id = k.namespace_id
		WHERE k.id = ? AND k.namespace_id = ?
		  AND n.owner_principal_id = ? AND n.status = 'active'`,
		keyID, namespaceID, principalID)
	if err != nil {
		return fmt.Errorf("delete API key: %w", err)
	}
	return requireAffected(result)
}

func (s *Store) ListAPIKeys(
	ctx context.Context,
	principalID, namespaceID string,
) ([]domain.APIKey, error) {
	if _, err := s.GetNamespace(ctx, principalID, namespaceID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, key_mask, permission, last_used_at, created_at
		FROM api_key
		WHERE namespace_id = ?
		ORDER BY created_at DESC`, namespaceID)
	if err != nil {
		return nil, fmt.Errorf("list API keys: %w", err)
	}
	defer rows.Close()

	keys := make([]domain.APIKey, 0)
	for rows.Next() {
		var key domain.APIKey
		if err := rows.Scan(
			&key.ID, &key.Name, &key.Mask, &key.Permission, &key.LastUsedAt, &key.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan API key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate API keys: %w", err)
	}
	return keys, nil
}

func (s *Store) FindCredential(ctx context.Context, secretHash []byte) (domain.Credential, error) {
	var credential domain.Credential
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.account_namespace, p.id, n.id, n.name,
		       COALESCE(p.iam_user_name, ''), k.id, k.permission
		FROM api_key k
		JOIN namespace n ON n.id = k.namespace_id
		JOIN iam_principal p ON p.id = n.owner_principal_id
		JOIN iam_account a ON a.id = n.account_id
		WHERE k.secret_hash = ?
		  AND a.status = 'active'
		  AND p.status = 'active'
		  AND n.status = 'active'`, secretHash).Scan(
		&credential.AccountID,
		&credential.AccountNamespace,
		&credential.PrincipalID,
		&credential.NamespaceID,
		&credential.Namespace,
		&credential.IAMUserName,
		&credential.APIKeyID,
		&credential.Permission,
	)
	if err != nil {
		return domain.Credential{}, mapQueryError("find API credential", err)
	}
	return credential, nil
}

func (s *Store) TouchAPIKey(ctx context.Context, keyID string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE api_key
		SET last_used_at = ?
		WHERE id = ? AND (last_used_at IS NULL OR last_used_at < ?)`,
		now.UTC(), keyID, now.UTC().Add(-time.Hour))
	if err != nil {
		return fmt.Errorf("touch API key: %w", err)
	}
	return nil
}

func (s *Store) CreateNamespace(
	ctx context.Context,
	namespace domain.NewNamespace,
) (domain.Namespace, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO namespace (
			id, account_id, owner_principal_id, name, display_name, is_default
		)
		SELECT ?, p.account_id, p.id, ?, ?, 0
		FROM iam_principal p
		JOIN iam_account a ON a.id = p.account_id
		WHERE p.id = ? AND p.account_id = ?
		  AND p.status = 'active' AND a.status = 'active'`,
		namespace.ID, namespace.Name, namespace.DisplayName,
		namespace.PrincipalID, namespace.AccountID)
	if err != nil {
		return domain.Namespace{}, mapWriteError("insert namespace", err)
	}
	if err := requireAffected(result); err != nil {
		return domain.Namespace{}, store.ErrUnauthorized
	}
	return s.GetNamespace(ctx, namespace.PrincipalID, namespace.ID)
}

func (s *Store) GetNamespace(
	ctx context.Context,
	principalID, namespaceID string,
) (domain.Namespace, error) {
	return scanNamespace(s.db.QueryRowContext(ctx, `
		SELECT id, name, display_name, is_default, created_at, updated_at
		FROM namespace
		WHERE id = ? AND owner_principal_id = ? AND status = 'active'`,
		namespaceID, principalID))
}

func (s *Store) ListNamespaces(
	ctx context.Context,
	principalID string,
) ([]domain.Namespace, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, display_name, is_default, created_at, updated_at
		FROM namespace
		WHERE owner_principal_id = ? AND status = 'active'
		ORDER BY is_default DESC, created_at ASC`, principalID)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	defer rows.Close()

	values := make([]domain.Namespace, 0)
	for rows.Next() {
		value, err := scanNamespace(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate namespaces: %w", err)
	}
	return values, nil
}

func (s *Store) UpdateNamespace(
	ctx context.Context,
	principalID, namespaceID, displayName string,
) (domain.Namespace, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE namespace SET display_name = ?
		WHERE id = ? AND owner_principal_id = ? AND status = 'active'`,
		displayName, namespaceID, principalID)
	if err != nil {
		return domain.Namespace{}, fmt.Errorf("update namespace: %w", err)
	}
	return s.GetNamespace(ctx, principalID, namespaceID)
}

func (s *Store) DeleteNamespace(ctx context.Context, principalID, namespaceID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin namespace deletion: %w", err)
	}
	defer rollback(tx)

	var isDefault bool
	err = tx.QueryRowContext(ctx, `
		SELECT is_default FROM namespace
		WHERE id = ? AND owner_principal_id = ? AND status = 'active'
		FOR UPDATE`, namespaceID, principalID).Scan(&isDefault)
	if err != nil {
		return mapQueryError("load namespace for deletion", err)
	}
	if isDefault {
		return store.ErrDefaultNamespace
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM api_key WHERE namespace_id = ?`, namespaceID); err != nil {
		return fmt.Errorf("delete namespace API keys: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE namespace SET status = 'deleted'
		WHERE id = ? AND owner_principal_id = ? AND status = 'active'`,
		namespaceID, principalID)
	if err != nil {
		return fmt.Errorf("delete namespace: %w", err)
	}
	if err := requireAffected(result); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit namespace deletion: %w", err)
	}
	return nil
}

func lockNamespace(ctx context.Context, tx *sql.Tx, principalID, namespaceID string) error {
	var status string
	err := tx.QueryRowContext(ctx, `
		SELECT status FROM namespace
		WHERE id = ? AND owner_principal_id = ?
		FOR UPDATE`, namespaceID, principalID).Scan(&status)
	if err != nil {
		return mapQueryError("lock namespace", err)
	}
	if status != "active" {
		return store.ErrNotFound
	}
	return nil
}

func queryAPIKey(ctx context.Context, tx *sql.Tx, keyID string) (domain.APIKey, error) {
	var key domain.APIKey
	err := tx.QueryRowContext(ctx, `
		SELECT id, name, key_mask, permission, last_used_at, created_at
		FROM api_key WHERE id = ?`, keyID).Scan(
		&key.ID, &key.Name, &key.Mask, &key.Permission, &key.LastUsedAt, &key.CreatedAt,
	)
	if err != nil {
		return domain.APIKey{}, mapQueryError("load API key", err)
	}
	return key, nil
}

type scanner interface {
	Scan(...any) error
}

func scanNamespace(row scanner) (domain.Namespace, error) {
	var value domain.Namespace
	if err := row.Scan(
		&value.ID, &value.Name, &value.DisplayName, &value.Default,
		&value.CreatedAt, &value.UpdatedAt,
	); err != nil {
		return domain.Namespace{}, mapQueryError("load namespace", err)
	}
	return value, nil
}

func requireAffected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}
	if rows == 0 {
		return store.ErrNotFound
	}
	return nil
}

func mapQueryError(operation string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func mapWriteError(operation string, err error) error {
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return store.ErrConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}
