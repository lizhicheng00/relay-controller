RENAME TABLE billing_usage_10m TO billing_usage_1m;

ALTER TABLE tunnel_usage_window
    MODIFY COLUMN window_start BIGINT UNSIGNED NOT NULL COMMENT 'one-minute UTC window start',
    COMMENT = 'Gateway cumulative one-minute traffic window';

ALTER TABLE billing_usage_1m
    MODIFY COLUMN window_start BIGINT UNSIGNED NOT NULL COMMENT 'one-minute UTC window start',
    COMMENT = 'Settled one-minute tunnel usage';
