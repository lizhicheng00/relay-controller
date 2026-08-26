START TRANSACTION;

CREATE TEMPORARY TABLE legacy_identity (
    identity_type VARCHAR(8) NOT NULL,
    domain_fingerprint BINARY(32) NOT NULL,
    user_fingerprint BINARY(32) NOT NULL,
    namespace VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    PRIMARY KEY (domain_fingerprint, user_fingerprint),
    UNIQUE KEY uk_legacy_identity_namespace (namespace)
);

-- Add precomputed HMAC-SHA256 fingerprints before production deployment:
-- INSERT INTO legacy_identity (identity_type, domain_fingerprint, user_fingerprint, namespace) VALUES
--     ('main', UNHEX('<domain-fingerprint>'), UNHEX('<user-fingerprint>'), 'namespace'),
--     ('sub', UNHEX('<domain-fingerprint>'), UNHEX('<user-fingerprint>'), 'ns-sub-namespace');

INSERT INTO domain_account (id, domain_fingerprint, account_namespace)
SELECT
    CONCAT('acc_', LEFT(LOWER(HEX(domain_fingerprint)), 28)),
    domain_fingerprint,
    namespace
FROM legacy_identity
WHERE identity_type = 'main';

INSERT INTO user_identity (account_id, user_fingerprint, namespace)
SELECT
    CONCAT('acc_', LEFT(LOWER(HEX(domain_fingerprint)), 28)),
    user_fingerprint,
    namespace
FROM legacy_identity;

DROP TEMPORARY TABLE legacy_identity;

COMMIT;
