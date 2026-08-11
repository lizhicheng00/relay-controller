INSERT INTO cluster (cluster_id, region, created_at, updated_at)
VALUES ('cn-north-4-bridge', 'cn-north-4', UNIX_TIMESTAMP(), UNIX_TIMESTAMP())
ON DUPLICATE KEY UPDATE region = 'cn-north-4', updated_at = UNIX_TIMESTAMP();
