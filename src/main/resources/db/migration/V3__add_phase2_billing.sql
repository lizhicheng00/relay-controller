CREATE TABLE billing_plan (
    plan_code VARCHAR(32) NOT NULL COMMENT 'stable plan identifier',
    monthly_quota_bytes BIGINT UNSIGNED NOT NULL COMMENT 'monthly traffic quota',
    max_tunnels SMALLINT UNSIGNED NOT NULL COMMENT 'maximum active tunnels',
    max_ports_per_tunnel SMALLINT UNSIGNED NOT NULL COMMENT 'maximum ports per tunnel',
    max_hosts_per_tunnel SMALLINT UNSIGNED NOT NULL COMMENT 'maximum concurrent hosts per tunnel',
    max_tunnel_bandwidth_bytes_per_second BIGINT UNSIGNED NOT NULL COMMENT 'bandwidth limit per tunnel',
    max_http_requests_per_minute_per_port INT UNSIGNED NOT NULL COMMENT 'HTTP request limit per port',
    max_connections_per_port INT UNSIGNED NOT NULL COMMENT 'concurrent connection limit per port',
    created_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'created unix seconds',
    updated_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'updated unix seconds',
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
    max_connections_per_port,
    created_at,
    updated_at
)
VALUES ('trial', 5368709120, 10, 10, 1, 5242880, 500, 100, UNIX_TIMESTAMP(), UNIX_TIMESTAMP())
ON DUPLICATE KEY UPDATE
    monthly_quota_bytes = VALUES(monthly_quota_bytes),
    max_tunnels = VALUES(max_tunnels),
    max_ports_per_tunnel = VALUES(max_ports_per_tunnel),
    max_hosts_per_tunnel = VALUES(max_hosts_per_tunnel),
    max_tunnel_bandwidth_bytes_per_second = VALUES(max_tunnel_bandwidth_bytes_per_second),
    max_http_requests_per_minute_per_port = VALUES(max_http_requests_per_minute_per_port),
    max_connections_per_port = VALUES(max_connections_per_port),
    updated_at = VALUES(updated_at);

CREATE TABLE billing_account (
    _id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    namespace VARCHAR(128) NOT NULL COMMENT 'account namespace',
    plan_code VARCHAR(32) NOT NULL DEFAULT 'trial' COMMENT 'billing plan identifier',
    status VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'active/disabled',
    created_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'created unix seconds',
    updated_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'updated unix seconds',
    PRIMARY KEY (_id),
    UNIQUE KEY uk_billing_account_namespace (namespace),
    KEY idx_billing_account_plan_code (plan_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Namespace billing account';

INSERT INTO billing_account (namespace, plan_code, status, created_at, updated_at)
SELECT DISTINCT namespace, 'trial', 'active', UNIX_TIMESTAMP(), UNIX_TIMESTAMP()
FROM tunnel
ON DUPLICATE KEY UPDATE updated_at = VALUES(updated_at);

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
    _id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    account_id BIGINT UNSIGNED NOT NULL COMMENT 'billing account id',
    period_start BIGINT UNSIGNED NOT NULL COMMENT 'inclusive UTC period start',
    period_end BIGINT UNSIGNED NOT NULL COMMENT 'exclusive UTC period end',
    quota_bytes BIGINT UNSIGNED NOT NULL COMMENT 'quota snapshot for this period',
    billed_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'settled traffic bytes',
    created_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'created unix seconds',
    updated_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'updated unix seconds',
    PRIMARY KEY (_id),
    UNIQUE KEY uk_billing_period_account_start (account_id, period_start),
    KEY idx_billing_period_end (period_end)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Monthly UTC billing period';

CREATE TABLE tunnel_usage_window (
    _id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    account_id BIGINT UNSIGNED NOT NULL COMMENT 'billing account id',
    cluster_id VARCHAR(128) NOT NULL COMMENT 'cluster identifier',
    tunnel_id VARCHAR(32) NOT NULL COMMENT 'base32 tunnel id',
    session_id VARCHAR(128) NOT NULL COMMENT 'host connection session id',
    window_start BIGINT UNSIGNED NOT NULL COMMENT '10-minute UTC window start',
    usage_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'cumulative bytes in this window',
    billed_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'bytes already settled',
    session_ended TINYINT NOT NULL DEFAULT 0 COMMENT 'host session ended flag',
    reported_at BIGINT UNSIGNED NOT NULL COMMENT 'latest gateway report unix seconds',
    created_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'created unix seconds',
    updated_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'updated unix seconds',
    PRIMARY KEY (_id),
    UNIQUE KEY uk_usage_tunnel_session_window (tunnel_id, session_id, window_start),
    KEY idx_usage_account_window (account_id, window_start),
    KEY idx_usage_cluster_unbilled (cluster_id, billed_bytes, window_start)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Gateway cumulative 10-minute traffic window';

CREATE TABLE billing_usage_10m (
    _id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    account_id BIGINT UNSIGNED NOT NULL COMMENT 'billing account id',
    tunnel_id VARCHAR(32) NOT NULL COMMENT 'base32 tunnel id',
    window_start BIGINT UNSIGNED NOT NULL COMMENT '10-minute UTC window start',
    usage_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'settled traffic bytes',
    created_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'created unix seconds',
    updated_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'updated unix seconds',
    PRIMARY KEY (_id),
    UNIQUE KEY uk_bill_account_tunnel_window (account_id, tunnel_id, window_start),
    KEY idx_bill_account_window (account_id, window_start)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Settled 10-minute tunnel usage';

CREATE TABLE tunnel_runtime_status (
    tunnel_id VARCHAR(32) NOT NULL COMMENT 'base32 tunnel id',
    cluster_id VARCHAR(128) NOT NULL COMMENT 'cluster identifier',
    gateway_id VARCHAR(128) NOT NULL COMMENT 'reporting gateway identifier',
    session_id VARCHAR(128) DEFAULT NULL COMMENT 'host connection session id',
    host_connections INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'host connection count',
    client_connections INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'client connection count',
    channel_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'active channel count',
    upload_bytes_per_second BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'current upload rate',
    download_bytes_per_second BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'current download rate',
    reported_at BIGINT UNSIGNED NOT NULL COMMENT 'gateway report unix seconds',
    updated_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'controller update unix seconds',
    PRIMARY KEY (tunnel_id),
    KEY idx_runtime_cluster_reported (cluster_id, reported_at),
    KEY idx_runtime_gateway (gateway_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Latest gateway tunnel runtime status';
