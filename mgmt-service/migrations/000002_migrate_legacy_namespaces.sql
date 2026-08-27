START TRANSACTION;

CREATE TEMPORARY TABLE legacy_identity (
    identity_type VARCHAR(8) NOT NULL,
    account_mapping_key BINARY(32) NOT NULL,
    member_mapping_key BINARY(32) NOT NULL,
    namespace VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    PRIMARY KEY (account_mapping_key, member_mapping_key),
    UNIQUE KEY uk_legacy_identity_namespace (namespace)
);

-- Generate the rows with: go run ./tools/identity-cutover
-- Add the generated HMAC-SHA256 mapping keys before production deployment:
-- INSERT INTO legacy_identity (identity_type, account_mapping_key, member_mapping_key, namespace) VALUES
--     ('main', UNHEX('<account-mapping-key>'), UNHEX('<member-mapping-key>'), 'namespace'),
--     ('sub', UNHEX('<account-mapping-key>'), UNHEX('<member-mapping-key>'), 'ns-sub-namespace');

INSERT INTO domain_account (id, mapping_key, account_namespace)
SELECT
    CONCAT('acc_', LEFT(LOWER(HEX(account_mapping_key)), 28)),
    account_mapping_key,
    namespace
FROM legacy_identity
WHERE identity_type = 'main';

INSERT INTO user_identity (account_id, mapping_key, namespace)
SELECT
    CONCAT('acc_', LEFT(LOWER(HEX(account_mapping_key)), 28)),
    member_mapping_key,
    namespace
FROM legacy_identity;

DROP TEMPORARY TABLE legacy_identity;

COMMIT;
