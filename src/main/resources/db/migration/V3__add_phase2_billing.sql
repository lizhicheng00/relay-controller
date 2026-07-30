CREATE TABLE billing_plan (
    plan_code VARCHAR(32) NOT NULL COMMENT 'stable plan identifier',
    monthly_quota_bytes BIGINT UNSIGNED NOT NULL COMMENT 'monthly traffic quota',
    max_tunnels SMALLINT UNSIGNED NOT NULL COMMENT 'maximum active tunnels',
    max_ports_per_tunnel SMALLINT UNSIGNED NOT NULL COMMENT 'maximum ports per tunnel',
    max_hosts_per_tunnel SMALLINT UNSIGNED NOT NULL COMMENT 'maximum concurrent hosts per tunnel',
    max_tunnel_bandwidth_bytes_per_second BIGINT UNSIGNED NOT NULL COMMENT 'bandwidth limit per tunnel',
    max_http_requests_per_minute_per_port INT UNSIGNED NOT NULL COMMENT 'HTTP request limit per port',
    max_connections_per_port INT UNSIGNED NOT NULL COMMENT 'concurrent connection limit per port',
    PRIMARY KEY (plan_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Billing and data-plane limit plan';

INSERT INTO billing_plan (
    plan_code,
    monthly_quota_bytes,
    max_tunnels,
    max_ports_per_tunnel,
    max_hosts_per_tunnel,
    max_tunnel_bandwidth_bytes_per_second,
    max_http_requests_per_minute_per_port,
    max_connections_per_port
)
VALUES ('trial', 5368709120, 10, 10, 1, 5242880, 500, 100)
ON DUPLICATE KEY UPDATE
    monthly_quota_bytes = VALUES(monthly_quota_bytes),
    max_tunnels = VALUES(max_tunnels),
    max_ports_per_tunnel = VALUES(max_ports_per_tunnel),
    max_hosts_per_tunnel = VALUES(max_hosts_per_tunnel),
    max_tunnel_bandwidth_bytes_per_second = VALUES(max_tunnel_bandwidth_bytes_per_second),
    max_http_requests_per_minute_per_port = VALUES(max_http_requests_per_minute_per_port),
    max_connections_per_port = VALUES(max_connections_per_port);

CREATE TABLE billing_account (
    _id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    namespace VARCHAR(128) NOT NULL COMMENT 'account namespace',
    plan_code VARCHAR(32) NOT NULL DEFAULT 'trial' COMMENT 'billing plan identifier',
    status VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'active/disabled',
    quota_blocked_until BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'traffic blocked before this unix second',
    PRIMARY KEY (_id),
    UNIQUE KEY uk_billing_account_namespace (namespace),
    KEY idx_billing_account_plan_code (plan_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Namespace billing account';

INSERT IGNORE INTO billing_account (namespace, plan_code, status)
SELECT DISTINCT namespace, 'trial', 'active'
FROM tunnel;

ALTER TABLE tunnel
    ADD COLUMN account_id BIGINT UNSIGNED NULL COMMENT 'billing account id' AFTER namespace,
    ADD KEY idx_tunnel_account_id (account_id);

UPDATE tunnel t
INNER JOIN billing_account a ON a.namespace = t.namespace
SET t.account_id = a._id
WHERE t.account_id IS NULL;

ALTER TABLE tunnel
    MODIFY COLUMN account_id BIGINT UNSIGNED NOT NULL COMMENT 'billing account id';

CREATE TABLE billing_period (
    account_id BIGINT UNSIGNED NOT NULL COMMENT 'billing account id',
    period_start BIGINT UNSIGNED NOT NULL COMMENT 'inclusive UTC period start',
    period_end BIGINT UNSIGNED NOT NULL COMMENT 'exclusive UTC period end',
    quota_bytes BIGINT UNSIGNED NOT NULL COMMENT 'quota snapshot for this period',
    billed_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'settled traffic bytes',
    PRIMARY KEY (account_id, period_start)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Monthly UTC billing period';

CREATE TABLE tunnel_metering (
    _id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    account_id BIGINT UNSIGNED NOT NULL COMMENT 'billing account id',
    cluster_id VARCHAR(128) NOT NULL COMMENT 'cluster identifier',
    tunnel_id VARCHAR(32) NOT NULL COMMENT 'base32 tunnel id',
    session_id VARCHAR(128) NOT NULL COMMENT 'host connection session id',
    usage_bytes BIGINT UNSIGNED NOT NULL COMMENT 'incremental bytes in this report',
    reported_at BIGINT UNSIGNED NOT NULL COMMENT 'gateway report unix seconds',
    created_at BIGINT UNSIGNED NOT NULL COMMENT 'database write unix seconds',
    settled TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0 pending, 1 settled',
    PRIMARY KEY (_id, reported_at),
    UNIQUE KEY uk_metering_report (tunnel_id, session_id, reported_at),
    KEY idx_metering_settlement (settled, created_at, _id),
    KEY idx_metering_account_reported (account_id, reported_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Gateway append-only traffic metering'
PARTITION BY RANGE (reported_at) (
    PARTITION p_future VALUES LESS THAN MAXVALUE
);

CREATE TABLE billing_usage_1m (
    account_id BIGINT UNSIGNED NOT NULL COMMENT 'billing account id',
    tunnel_id VARCHAR(32) NOT NULL COMMENT 'base32 tunnel id',
    window_start BIGINT UNSIGNED NOT NULL COMMENT 'one-minute UTC window start',
    usage_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'settled traffic bytes',
    PRIMARY KEY (account_id, tunnel_id, window_start),
    KEY idx_bill_account_window (account_id, window_start)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Settled one-minute tunnel usage';

CREATE TABLE tunnel_runtime_status (
    tunnel_id VARCHAR(32) NOT NULL COMMENT 'base32 tunnel id',
    host_connection_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'active host connection count',
    client_connection_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'active SSH channel count',
    upload_bytes_per_second BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'current upload rate',
    download_bytes_per_second BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'current download rate',
    reported_at BIGINT UNSIGNED NOT NULL COMMENT 'gateway report unix seconds',
    PRIMARY KEY (tunnel_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Latest gateway tunnel runtime status';

DROP TABLE IF EXISTS metering;
