CREATE TABLE IF NOT EXISTS domain_account (
    id VARCHAR(32) COLLATE utf8mb4_bin NOT NULL,
    domain_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    account_namespace VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uk_domain_account_domain (domain_id),
    UNIQUE KEY uk_domain_account_namespace (account_namespace)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_identity (
    account_id VARCHAR(32) COLLATE utf8mb4_bin NOT NULL,
    user_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    namespace VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (account_id, user_id),
    UNIQUE KEY uk_user_identity_namespace (namespace),
    CONSTRAINT fk_user_identity_account
        FOREIGN KEY (account_id) REFERENCES domain_account (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS api_key (
    id VARCHAR(32) COLLATE utf8mb4_bin NOT NULL,
    namespace VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    slot TINYINT UNSIGNED NOT NULL,
    name VARCHAR(64) NOT NULL,
    scenario VARCHAR(16) COLLATE utf8mb4_bin NOT NULL,
    key_mask VARCHAR(32) NOT NULL,
    key_hash BINARY(32) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uk_api_key_namespace_slot (namespace, slot),
    UNIQUE KEY uk_api_key_namespace_name (namespace, name),
    UNIQUE KEY uk_api_key_hash (key_hash),
    CONSTRAINT chk_api_key_slot CHECK (slot <= 4),
    CONSTRAINT chk_api_key_scenario CHECK (scenario IN ('devbridge', 'devbox')),
    CONSTRAINT fk_api_key_identity
        FOREIGN KEY (namespace) REFERENCES user_identity (namespace)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
