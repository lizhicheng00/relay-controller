START TRANSACTION;

CREATE TEMPORARY TABLE legacy_identity (
    identity_type VARCHAR(8) NOT NULL,
    customer_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    user_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    namespace VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    PRIMARY KEY (customer_id, user_id),
    UNIQUE KEY uk_legacy_identity_namespace (namespace)
);

-- Add the main and sub-user mappings before production deployment:
-- INSERT INTO legacy_identity (identity_type, customer_id, user_id, namespace) VALUES
--     ('main', 'domain-id', 'main-user-id', 'namespace'),
--     ('sub', 'domain-id', 'sub-user-id', 'ns-sub-namespace');

INSERT INTO domain_account (id, domain_id, account_namespace)
SELECT
    CONCAT('acc_', LEFT(SHA2(CONCAT('domain:', customer_id), 256), 28)),
    customer_id,
    namespace
FROM legacy_identity
WHERE identity_type = 'main';

INSERT INTO user_identity (account_id, user_id, namespace)
SELECT
    CONCAT('acc_', LEFT(SHA2(CONCAT('domain:', customer_id), 256), 28)),
    user_id,
    namespace
FROM legacy_identity;

DROP TEMPORARY TABLE legacy_identity;

COMMIT;
