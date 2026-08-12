package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/lizhicheng00/relay-controller/relay-controller/internal/core"
)

func (s *Store) CreateAccountIfAbsent(ctx context.Context, namespace, planCode string) error {
	_, err := s.exec.ExecContext(ctx, `INSERT IGNORE INTO billing_account (namespace, plan_code, status)
		VALUES (?, ?, 'active')`, namespace, planCode)
	return err
}

func (s *Store) LockAccountByNamespace(ctx context.Context, namespace string) (core.BillingAccount, error) {
	return scanAccount(s.exec.QueryRowContext(ctx, `SELECT _id, plan_code, status
		FROM billing_account WHERE namespace = ? LIMIT 1 FOR UPDATE`, namespace))
}

func (s *Store) LockAccountByID(ctx context.Context, accountID uint64) (core.BillingAccount, error) {
	return scanAccount(s.exec.QueryRowContext(ctx, `SELECT _id, plan_code, status
		FROM billing_account WHERE _id = ? LIMIT 1 FOR UPDATE`, accountID))
}

func (s *Store) FindPlan(ctx context.Context, planCode string) (core.BillingPlan, error) {
	var plan core.BillingPlan
	err := s.exec.QueryRowContext(ctx, `SELECT monthly_quota_bytes, max_tunnels,
		max_ports_per_tunnel, max_hosts_per_tunnel, max_tunnel_bandwidth_bytes_per_second,
		max_http_requests_per_minute_per_port, max_connections_per_port
		FROM billing_plan WHERE plan_code = ? LIMIT 1`, planCode).Scan(
		&plan.MonthlyQuotaBytes, &plan.MaxTunnels, &plan.MaxPortsPerTunnel,
		&plan.MaxHostsPerTunnel, &plan.MaxTunnelBandwidthBytesPerSecond,
		&plan.MaxHTTPRequestsPerMinutePerPort, &plan.MaxConnectionsPerPort)
	return plan, err
}

func (s *Store) CreatePeriodIfAbsent(ctx context.Context, accountID uint64, start, end int64, quota uint64) error {
	_, err := s.exec.ExecContext(ctx, `INSERT IGNORE INTO billing_period
		(account_id, period_start, period_end, quota_bytes, billed_bytes) VALUES (?, ?, ?, ?, 0)`,
		accountID, start, end, quota)
	return err
}

func (s *Store) FindPeriod(ctx context.Context, accountID uint64, start int64) (core.BillingPeriod, error) {
	var period core.BillingPeriod
	err := s.exec.QueryRowContext(ctx, `SELECT period_end, quota_bytes, billed_bytes
		FROM billing_period WHERE account_id = ? AND period_start = ? LIMIT 1`, accountID, start).Scan(
		&period.End, &period.QuotaBytes, &period.BilledBytes)
	return period, err
}

func (s *Store) IncreasePeriodUsage(ctx context.Context, accountID uint64, start int64, usage uint64) error {
	result, err := s.exec.ExecContext(ctx, `UPDATE billing_period SET billed_bytes = billed_bytes + ?
		WHERE account_id = ? AND period_start = ?`, usage, accountID, start)
	if err != nil {
		return err
	}
	return requireOneRow(result, "update billing period")
}

func (s *Store) BlockQuotaIfExhausted(ctx context.Context, accountID uint64, start int64) error {
	_, err := s.exec.ExecContext(ctx, `UPDATE billing_account a
		INNER JOIN billing_period bp ON bp.account_id = a._id AND bp.period_start = ?
		SET a.quota_blocked_until = GREATEST(a.quota_blocked_until, bp.period_end)
		WHERE a._id = ? AND bp.billed_bytes >= bp.quota_bytes AND a.quota_blocked_until < bp.period_end`,
		start, accountID)
	return err
}

func (s *Store) LockUnsettledMetering(ctx context.Context, clusterIDs []string, limit int) ([]core.MeteringRecord, error) {
	if len(clusterIDs) == 0 {
		return nil, nil
	}
	arguments := make([]any, 0, len(clusterIDs)+1)
	for _, clusterID := range clusterIDs {
		arguments = append(arguments, clusterID)
	}
	arguments = append(arguments, limit)
	rows, err := s.exec.QueryContext(ctx, `SELECT m._id, m.account_id, m.tunnel_id,
		m.upload_bytes, m.download_bytes, m.reported_at
		FROM tunnel_metering m FORCE INDEX (idx_metering_settlement)
		WHERE m.settled = 0 AND m.cluster_id IN (`+placeholders(len(clusterIDs))+`)
		ORDER BY m.created_at ASC, m._id ASC LIMIT ? FOR UPDATE SKIP LOCKED`, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]core.MeteringRecord, 0)
	for rows.Next() {
		var record core.MeteringRecord
		if err := rows.Scan(&record.ID, &record.AccountID, &record.TunnelID, &record.UploadBytes,
			&record.DownloadBytes, &record.ReportedAt); err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

func (s *Store) MarkMeteringSettled(ctx context.Context, records []core.MeteringRecord) (int64, error) {
	if len(records) == 0 {
		return 0, nil
	}
	values := make([]string, len(records))
	arguments := make([]any, 0, len(records)*2)
	for index, record := range records {
		values[index] = "(?, ?)"
		arguments = append(arguments, record.ID, record.ReportedAt)
	}
	result, err := s.exec.ExecContext(ctx, `UPDATE tunnel_metering SET settled = 1
		WHERE settled = 0 AND (_id, reported_at) IN (`+strings.Join(values, ",")+`)`, arguments...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

type Partition struct {
	Name     string
	Boundary int64
}

func (s *Store) MeteringPartitions(ctx context.Context) ([]Partition, error) {
	rows, err := s.exec.QueryContext(ctx, `SELECT partition_name, partition_description
		FROM information_schema.partitions
		WHERE table_schema = DATABASE() AND table_name = 'tunnel_metering'
		AND partition_name IS NOT NULL AND partition_description != 'MAXVALUE'
		ORDER BY partition_ordinal_position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Partition
	for rows.Next() {
		var partition Partition
		if err := rows.Scan(&partition.Name, &partition.Boundary); err != nil {
			return nil, err
		}
		result = append(result, partition)
	}
	return result, rows.Err()
}

func (s *Store) ReorganizeFuturePartition(ctx context.Context, definitions []string) error {
	if len(definitions) == 0 {
		return nil
	}
	definitions = append(definitions, "PARTITION p_future VALUES LESS THAN MAXVALUE")
	_, err := s.exec.ExecContext(ctx, `ALTER TABLE tunnel_metering REORGANIZE PARTITION p_future INTO (`+
		strings.Join(definitions, ", ")+`)`)
	return err
}

func (s *Store) DropMeteringPartitions(ctx context.Context, names []string) error {
	if len(names) == 0 {
		return nil
	}
	for _, name := range names {
		if !validPartitionName(name) {
			return fmt.Errorf("invalid partition name %q", name)
		}
	}
	_, err := s.exec.ExecContext(ctx, `ALTER TABLE tunnel_metering DROP PARTITION `+strings.Join(names, ", "))
	return err
}

func scanAccount(row scanner) (core.BillingAccount, error) {
	var account core.BillingAccount
	err := row.Scan(&account.ID, &account.PlanCode, &account.Status)
	return account, err
}

func validPartitionName(name string) bool {
	if !strings.HasPrefix(name, "p_") || len(name) != 12 {
		return false
	}
	for _, character := range name[2:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
