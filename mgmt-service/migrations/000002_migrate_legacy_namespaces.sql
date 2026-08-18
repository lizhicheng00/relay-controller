START TRANSACTION;

CREATE TEMPORARY TABLE legacy_identity (
    identity_type VARCHAR(8) NOT NULL,
    customer_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    user_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    namespace VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    PRIMARY KEY (customer_id, user_id),
    UNIQUE KEY uk_legacy_identity_namespace (namespace)
);

-- Replace this row with all main and sub-user mappings before deployment.
INSERT INTO legacy_identity (identity_type, customer_id, user_id, namespace) VALUES
    ('main', '__REPLACE_CUSTOMER_ID__', '__REPLACE_USER_ID__', '__REPLACE_NAMESPACE__');

-- Fail closed rather than applying a migration containing placeholder data.
CREATE TEMPORARY TABLE migration_ready (
    ready TINYINT NOT NULL PRIMARY KEY
);

INSERT INTO migration_ready (ready) VALUES (1);

INSERT INTO migration_ready (ready)
SELECT 1
FROM legacy_identity
WHERE customer_id = '__REPLACE_CUSTOMER_ID__'
   OR user_id = '__REPLACE_USER_ID__'
   OR namespace = '__REPLACE_NAMESPACE__'
LIMIT 1;

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

DROP TEMPORARY TABLE migration_ready;
DROP TEMPORARY TABLE legacy_identity;

COMMIT;
