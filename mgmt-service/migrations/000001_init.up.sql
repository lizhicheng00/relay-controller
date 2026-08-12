CREATE TABLE iam_account (
    id VARCHAR(32) NOT NULL,
    iam_domain_id VARCHAR(128) NOT NULL,
    account_namespace VARCHAR(128) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uk_iam_account_domain (iam_domain_id),
    UNIQUE KEY uk_iam_account_namespace (account_namespace)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE iam_principal (
    id VARCHAR(32) NOT NULL,
    account_id VARCHAR(32) NOT NULL,
    iam_user_id VARCHAR(128) NOT NULL,
    iam_user_name VARCHAR(128) NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uk_iam_principal_user (account_id, iam_user_id),
    CONSTRAINT fk_principal_account FOREIGN KEY (account_id) REFERENCES iam_account (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE namespace (
    id VARCHAR(32) NOT NULL,
    account_id VARCHAR(32) NOT NULL,
    owner_principal_id VARCHAR(32) NOT NULL,
    name VARCHAR(128) NOT NULL,
    display_name VARCHAR(64) NOT NULL,
    is_default TINYINT UNSIGNED NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    default_owner_principal_id VARCHAR(32)
        GENERATED ALWAYS AS (CASE WHEN is_default = 1 THEN owner_principal_id ELSE NULL END) STORED,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uk_namespace_name (name),
    UNIQUE KEY uk_namespace_default_owner (default_owner_principal_id),
    KEY idx_namespace_owner (owner_principal_id, status, created_at),
    KEY idx_namespace_account (account_id),
    CONSTRAINT fk_namespace_account FOREIGN KEY (account_id) REFERENCES iam_account (id),
    CONSTRAINT fk_namespace_owner FOREIGN KEY (owner_principal_id) REFERENCES iam_principal (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE api_key (
    id VARCHAR(32) NOT NULL,
    namespace_id VARCHAR(32) NOT NULL,
    name VARCHAR(64) NOT NULL,
    key_mask VARCHAR(16) NOT NULL,
    secret_hash BINARY(32) NOT NULL,
    permission VARCHAR(16) NOT NULL DEFAULT 'write',
    last_used_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uk_api_key_hash (secret_hash),
    UNIQUE KEY uk_api_key_name (namespace_id, name),
    KEY idx_api_key_namespace_created (namespace_id, created_at),
    CONSTRAINT chk_api_key_permission CHECK (permission IN ('read', 'write')),
    CONSTRAINT fk_api_key_namespace FOREIGN KEY (namespace_id) REFERENCES namespace (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
