ALTER TABLE api_key
    DROP INDEX uk_api_key_namespace_slot,
    DROP INDEX uk_api_key_namespace_name,
    DROP CONSTRAINT chk_api_key_scenario,
    CHANGE COLUMN scenario key_type VARCHAR(16) COLLATE utf8mb4_bin NOT NULL,
    ADD COLUMN last_used_at DATETIME(6) NULL AFTER created_at,
    ADD UNIQUE KEY uk_api_key_namespace_type_slot (namespace, key_type, slot),
    ADD UNIQUE KEY uk_api_key_namespace_type_name (namespace, key_type, name),
    ADD CONSTRAINT chk_api_key_type CHECK (key_type IN ('devbridge', 'devbox'));
