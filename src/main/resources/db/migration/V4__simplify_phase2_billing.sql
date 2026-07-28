ALTER TABLE billing_plan
    DROP COLUMN created_at,
    DROP COLUMN updated_at;

ALTER TABLE billing_account
    DROP COLUMN created_at,
    DROP COLUMN updated_at;

ALTER TABLE billing_period
    DROP PRIMARY KEY,
    DROP INDEX uk_billing_period_account_start,
    DROP INDEX idx_billing_period_end,
    DROP COLUMN _id,
    DROP COLUMN created_at,
    DROP COLUMN updated_at,
    ADD PRIMARY KEY (account_id, period_start);

ALTER TABLE tunnel_usage_window
    DROP COLUMN session_ended,
    DROP COLUMN created_at,
    DROP COLUMN updated_at;

ALTER TABLE billing_usage_10m
    DROP PRIMARY KEY,
    DROP INDEX uk_bill_account_tunnel_window,
    DROP COLUMN _id,
    DROP COLUMN created_at,
    DROP COLUMN updated_at,
    ADD PRIMARY KEY (account_id, tunnel_id, window_start);

ALTER TABLE tunnel_runtime_status
    DROP COLUMN updated_at;
