ALTER TABLE tunnel_runtime_status
    ADD COLUMN total_upload_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0
        COMMENT 'gateway reported total upload bytes' AFTER download_bytes_per_second,
    ADD COLUMN total_download_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0
        COMMENT 'gateway reported total download bytes' AFTER total_upload_bytes;
