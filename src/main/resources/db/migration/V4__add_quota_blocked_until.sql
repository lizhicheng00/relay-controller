ALTER TABLE billing_account
    ADD COLUMN quota_blocked_until BIGINT UNSIGNED NOT NULL DEFAULT 0
        COMMENT 'traffic blocked before this unix second'
        AFTER status;

UPDATE billing_account a
INNER JOIN (
    SELECT account_id, MAX(period_end) AS blocked_until
    FROM billing_period
    WHERE billed_bytes >= quota_bytes
    GROUP BY account_id
) exhausted ON exhausted.account_id = a._id
SET a.quota_blocked_until = exhausted.blocked_until;
