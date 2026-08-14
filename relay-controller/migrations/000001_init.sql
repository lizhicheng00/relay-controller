CREATE TABLE IF NOT EXISTS cluster (
    _id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    cluster_id VARCHAR(128) NOT NULL COMMENT 'cluster identifier',
    region VARCHAR(128) NOT NULL COMMENT 'region',
    created_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'created unix seconds',
    updated_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'updated unix seconds',
    PRIMARY KEY (_id),
    UNIQUE KEY uk_cluster_id (cluster_id),
    KEY idx_region (region)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Cluster information';

CREATE TABLE IF NOT EXISTS billing_plan (
    plan_code VARCHAR(32) NOT NULL COMMENT 'stable plan identifier',
    monthly_quota_bytes BIGINT UNSIGNED NOT NULL COMMENT 'monthly traffic quota',
    max_tunnels SMALLINT UNSIGNED NOT NULL COMMENT 'maximum active tunnels',
    max_ports_per_tunnel SMALLINT UNSIGNED NOT NULL COMMENT 'maximum ports per tunnel',
    max_hosts_per_tunnel SMALLINT UNSIGNED NOT NULL COMMENT 'maximum concurrent hosts per tunnel',
    max_tunnel_bandwidth_bytes_per_second BIGINT UNSIGNED NOT NULL COMMENT 'bandwidth limit per tunnel',
    max_http_requests_per_minute_per_port INT UNSIGNED NOT NULL COMMENT 'HTTP request limit per port',
    max_connections_per_port INT UNSIGNED NOT NULL COMMENT 'concurrent connection limit per port',
    PRIMARY KEY (plan_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Billing and data-plane limit plan';

CREATE TABLE IF NOT EXISTS billing_account (
    _id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    namespace VARCHAR(128) NOT NULL COMMENT 'account namespace',
    plan_code VARCHAR(32) NOT NULL DEFAULT 'trial' COMMENT 'billing plan identifier',
    status VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'active/disabled',
    quota_blocked_until BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'traffic blocked before this unix second',
    PRIMARY KEY (_id),
    UNIQUE KEY uk_billing_account_namespace (namespace),
    KEY idx_billing_account_plan_code (plan_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Namespace billing account';

CREATE TABLE IF NOT EXISTS tunnel (
    _id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    name VARCHAR(128) NOT NULL COMMENT 'tunnel name',
    tunnel_id VARCHAR(32) NOT NULL COMMENT 'base32 encoded 40bit tunnel code',
    tunnel_code BIGINT UNSIGNED NOT NULL COMMENT '40bit tunnel code',
    cluster_id VARCHAR(128) NOT NULL COMMENT 'cluster identifier',
    expiration INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'expiration unix seconds',
    expiration_hours SMALLINT UNSIGNED NOT NULL DEFAULT 72 COMMENT 'configured expiration duration in hours',
    namespace VARCHAR(128) NOT NULL COMMENT 'namespace',
    account_id BIGINT UNSIGNED NOT NULL COMMENT 'billing account id',
    description VARCHAR(512) DEFAULT NULL COMMENT 'description',
    bandwidth_used BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'used bytes',
    url VARCHAR(512) NOT NULL COMMENT 'public url',
    type VARCHAR(64) NOT NULL DEFAULT 'bridge' COMMENT 'tunnel type: bridge/env',
    deleted TINYINT NOT NULL DEFAULT 0 COMMENT 'soft delete flag',
    created_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'created unix seconds',
    updated_at BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'updated unix seconds',
    PRIMARY KEY (_id),
    UNIQUE KEY uk_tunnel_id (tunnel_id),
    UNIQUE KEY uk_tunnel_code (tunnel_code),
    UNIQUE KEY uk_tunnel_namespace_name (namespace, name),
    KEY idx_namespace (namespace),
    KEY idx_cluster_id (cluster_id),
    KEY idx_namespace_deleted (namespace, deleted),
    KEY idx_tunnel_account_id (account_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tunnel metadata';

CREATE TABLE IF NOT EXISTS tunnel_port (
    _id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    tunnel_code BIGINT UNSIGNED NOT NULL COMMENT 'tunnel code',
    port BIGINT UNSIGNED NOT NULL COMMENT 'port, business range 1-65535',
    protocol VARCHAR(16) NOT NULL DEFAULT 'auto' COMMENT 'application protocol: http/https/auto',
    allow_anonymous TINYINT NOT NULL DEFAULT 0 COMMENT 'allow anonymous access',
    PRIMARY KEY (_id),
    UNIQUE KEY uk_tunnel_code_port (tunnel_code, port),
    KEY idx_tunnel_code (tunnel_code),
    KEY idx_port (port)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tunnel port policy';

CREATE TABLE IF NOT EXISTS billing_period (
    account_id BIGINT UNSIGNED NOT NULL COMMENT 'billing account id',
    period_start BIGINT UNSIGNED NOT NULL COMMENT 'inclusive Asia/Shanghai period start as unix seconds',
    period_end BIGINT UNSIGNED NOT NULL COMMENT 'exclusive Asia/Shanghai period end as unix seconds',
    quota_bytes BIGINT UNSIGNED NOT NULL COMMENT 'quota snapshot for this period',
    billed_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'settled traffic bytes',
    PRIMARY KEY (account_id, period_start)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Monthly billing period with Asia/Shanghai boundaries';

CREATE TABLE IF NOT EXISTS tunnel_metering (
    _id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    account_id BIGINT UNSIGNED NOT NULL COMMENT 'billing account id',
    cluster_id VARCHAR(128) NOT NULL COMMENT 'cluster identifier',
    tunnel_id VARCHAR(32) NOT NULL COMMENT 'base32 tunnel id',
    session_id VARCHAR(128) NOT NULL COMMENT 'host connection session id',
    upload_bytes BIGINT UNSIGNED NOT NULL COMMENT 'incremental upload bytes in this report',
    download_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'incremental download bytes in this report',
    reported_at BIGINT UNSIGNED NOT NULL COMMENT 'gateway report unix seconds',
    created_at BIGINT UNSIGNED NOT NULL COMMENT 'database write unix seconds',
    settled TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0 pending, 1 settled',
    PRIMARY KEY (_id, reported_at),
    UNIQUE KEY uk_metering_report (tunnel_id, session_id, reported_at),
    KEY idx_metering_settlement (settled, created_at, _id),
    KEY idx_metering_account_reported (account_id, reported_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Gateway append-only traffic metering'
PARTITION BY RANGE (reported_at) (
    PARTITION p_future VALUES LESS THAN MAXVALUE
);

CREATE TABLE IF NOT EXISTS tunnel_runtime_status (
    tunnel_id VARCHAR(32) NOT NULL COMMENT 'base32 tunnel id',
    host_connection_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'active host connection count',
    client_connection_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'active SSH channel count',
    upload_bytes_per_second BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'current upload rate',
    download_bytes_per_second BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'current download rate',
    total_upload_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'gateway reported total upload bytes',
    total_download_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'gateway reported total download bytes',
    reported_at BIGINT UNSIGNED NOT NULL COMMENT 'gateway report unix seconds',
    PRIMARY KEY (tunnel_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Latest gateway tunnel runtime status';

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

INSERT INTO cluster (cluster_id, region, created_at, updated_at)
VALUES ('cn-north-4-bridge', 'cn-north-4', UNIX_TIMESTAMP(), UNIX_TIMESTAMP())
ON DUPLICATE KEY UPDATE region = VALUES(region), updated_at = VALUES(updated_at);
