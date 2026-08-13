CREATE TABLE domain_account (
    id VARCHAR(32) COLLATE utf8mb4_bin NOT NULL,
    domain_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    account_namespace VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uk_domain_account_domain (domain_id),
    UNIQUE KEY uk_domain_account_namespace (account_namespace)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE user_identity (
    account_id VARCHAR(32) COLLATE utf8mb4_bin NOT NULL,
    user_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    namespace VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    api_key_hash BINARY(32) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (account_id, user_id),
    UNIQUE KEY uk_user_identity_namespace (namespace),
    UNIQUE KEY uk_user_identity_api_key (api_key_hash),
    CONSTRAINT fk_user_identity_account
        FOREIGN KEY (account_id) REFERENCES domain_account (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
