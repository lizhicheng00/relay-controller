package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"relay-controller/internal/core"
)

const tunnelColumns = `t._id, t.name, t.tunnel_id, t.tunnel_code, t.cluster_id,
	t.expiration, t.expiration_hours, t.namespace, t.account_id, t.description,
	t.bandwidth_used, t.url, t.type, t.created_at, t.updated_at`

const tunnelNameUniqueKey = "uk_tunnel_namespace_name"

type scanner interface {
	Scan(...any) error
}

func IsTunnelNameConflict(err error) bool {
	return isDuplicateKey(err, tunnelNameUniqueKey)
}

func (s *Store) ClusterExists(ctx context.Context, clusterID, region string) (bool, error) {
	var exists bool
	err := s.exec.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM cluster WHERE cluster_id = ? AND region = ?)`, clusterID, region).Scan(&exists)
	return exists, err
}

func (s *Store) LocalClusterIDs(ctx context.Context, region string) ([]string, error) {
	rows, err := s.exec.QueryContext(ctx, `SELECT cluster_id FROM cluster WHERE region = ? ORDER BY cluster_id`, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var clusterID string
		if err := rows.Scan(&clusterID); err != nil {
			return nil, err
		}
		result = append(result, clusterID)
	}
	return result, rows.Err()
}

func (s *Store) FindTunnel(ctx context.Context, tunnelID, region string) (core.Tunnel, error) {
	row := s.exec.QueryRowContext(ctx, `SELECT `+tunnelColumns+`
		FROM tunnel t INNER JOIN cluster c ON c.cluster_id = t.cluster_id
		WHERE t.tunnel_id = ? AND c.region = ? AND t.deleted = 0 LIMIT 1`, tunnelID, region)
	return scanTunnel(row, false)
}

func (s *Store) LockTunnel(ctx context.Context, tunnelID, region string) (core.Tunnel, error) {
	row := s.exec.QueryRowContext(ctx, `SELECT `+tunnelColumns+`
		FROM tunnel t INNER JOIN cluster c ON c.cluster_id = t.cluster_id
		WHERE t.tunnel_id = ? AND c.region = ? AND t.deleted = 0 LIMIT 1 FOR UPDATE`, tunnelID, region)
	return scanTunnel(row, false)
}

func (s *Store) ListActiveTunnels(ctx context.Context, namespace, clusterID, region string, now int64) ([]core.Tunnel, error) {
	query := `SELECT ` + tunnelColumns + `,
		(SELECT COUNT(*) FROM tunnel_port tp WHERE tp.tunnel_code = t.tunnel_code) AS port_count
		FROM tunnel t INNER JOIN cluster c ON c.cluster_id = t.cluster_id
		WHERE t.namespace = ? AND c.region = ? AND t.deleted = 0 AND t.expiration > ?`
	arguments := []any{namespace, region, now}
	if clusterID != "" {
		query += ` AND t.cluster_id = ?`
		arguments = append(arguments, clusterID)
	}
	query += ` ORDER BY t.created_at DESC`
	rows, err := s.exec.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]core.Tunnel, 0)
	for rows.Next() {
		tunnel, err := scanTunnel(rows, true)
		if err != nil {
			return nil, err
		}
		result = append(result, tunnel)
	}
	return result, rows.Err()
}

func (s *Store) LockNamespaceTunnels(ctx context.Context, namespace, region string) ([]core.Tunnel, error) {
	rows, err := s.exec.QueryContext(ctx, `SELECT `+tunnelColumns+`
		FROM tunnel t INNER JOIN cluster c ON c.cluster_id = t.cluster_id
		WHERE t.namespace = ? AND c.region = ? AND t.deleted = 0
		ORDER BY t.created_at DESC FOR UPDATE`, namespace, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []core.Tunnel
	for rows.Next() {
		tunnel, err := scanTunnel(rows, false)
		if err != nil {
			return nil, err
		}
		result = append(result, tunnel)
	}
	return result, rows.Err()
}

func (s *Store) CountAccountActiveTunnels(ctx context.Context, accountID uint64, now int64) (uint64, error) {
	var count uint64
	err := s.exec.QueryRowContext(ctx, `SELECT COUNT(*) FROM tunnel
		WHERE account_id = ? AND deleted = 0 AND expiration > ?`, accountID, now).Scan(&count)
	return count, err
}

func (s *Store) CountNamespaceActiveTunnels(ctx context.Context, namespace string, now int64) (uint64, error) {
	var count uint64
	err := s.exec.QueryRowContext(ctx, `SELECT COUNT(*) FROM tunnel
		WHERE namespace = ? AND deleted = 0 AND expiration > ?`, namespace, now).Scan(&count)
	return count, err
}

func (s *Store) InsertTunnel(ctx context.Context, tunnel *core.Tunnel) error {
	result, err := s.exec.ExecContext(ctx, `INSERT INTO tunnel (
		name, tunnel_id, tunnel_code, cluster_id, expiration, expiration_hours, namespace,
		account_id, description, bandwidth_used, url, type, deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		tunnel.Name, tunnel.TunnelID, tunnel.TunnelCode, tunnel.ClusterID, tunnel.Expiration,
		tunnel.ExpirationHours, tunnel.Namespace, tunnel.AccountID, tunnel.Description,
		tunnel.BandwidthUsed, tunnel.URL, tunnel.Type, tunnel.CreatedAt, tunnel.UpdatedAt)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	tunnel.ID = uint64(id)
	return nil
}

func (s *Store) UpdateTunnel(ctx context.Context, tunnel core.Tunnel) error {
	result, err := s.exec.ExecContext(ctx, `UPDATE tunnel SET
		name = ?, description = ?, expiration = ?, expiration_hours = ?, type = ?, updated_at = ?
		WHERE _id = ? AND deleted = 0`, tunnel.Name, tunnel.Description, tunnel.Expiration,
		tunnel.ExpirationHours, tunnel.Type, tunnel.UpdatedAt, tunnel.ID)
	if err != nil {
		return err
	}
	return requireOneRow(result, "update tunnel")
}

func (s *Store) DeleteTunnelGraph(ctx context.Context, tunnel core.Tunnel) error {
	if _, err := s.exec.ExecContext(ctx, `DELETE FROM tunnel_port WHERE tunnel_code = ?`, tunnel.TunnelCode); err != nil {
		return err
	}
	if _, err := s.exec.ExecContext(ctx, `DELETE FROM tunnel_runtime_status WHERE tunnel_id = ?`, tunnel.TunnelID); err != nil {
		return err
	}
	result, err := s.exec.ExecContext(ctx, `DELETE FROM tunnel WHERE _id = ?`, tunnel.ID)
	if err != nil {
		return err
	}
	return requireOneRow(result, "delete tunnel")
}

func (s *Store) RefreshTunnelExpiration(ctx context.Context, tunnelID, region string, activityAt int64) error {
	_, err := s.exec.ExecContext(ctx, `UPDATE tunnel t
		INNER JOIN cluster c ON c.cluster_id = t.cluster_id
		SET t.expiration = GREATEST(t.expiration, ? + t.expiration_hours * 3600)
		WHERE t.tunnel_id = ? AND c.region = ? AND t.deleted = 0 AND t.expiration > ?`,
		activityAt, tunnelID, region, activityAt)
	return err
}

func (s *Store) IncreaseTunnelUsage(ctx context.Context, tunnelID, region string, usage, updatedAt uint64) error {
	_, err := s.exec.ExecContext(ctx, `UPDATE tunnel t
		INNER JOIN cluster c ON c.cluster_id = t.cluster_id
		SET t.bandwidth_used = t.bandwidth_used + ?, t.updated_at = ?
		WHERE t.tunnel_id = ? AND c.region = ? AND t.deleted = 0`, usage, updatedAt, tunnelID, region)
	return err
}

func (s *Store) LockAgedTunnels(ctx context.Context, region string, cutoff int64, limit int) ([]core.Tunnel, error) {
	rows, err := s.exec.QueryContext(ctx, `SELECT `+tunnelColumns+`
		FROM tunnel t INNER JOIN cluster c ON c.cluster_id = t.cluster_id
		WHERE c.region = ? AND t.expiration <= ?
		ORDER BY t.expiration ASC LIMIT ? FOR UPDATE SKIP LOCKED`, region, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []core.Tunnel
	for rows.Next() {
		tunnel, err := scanTunnel(rows, false)
		if err != nil {
			return nil, err
		}
		result = append(result, tunnel)
	}
	return result, rows.Err()
}

func (s *Store) FindTunnelStatus(ctx context.Context, tunnelID string) (*core.TunnelStatus, error) {
	var status core.TunnelStatus
	err := s.exec.QueryRowContext(ctx, `SELECT host_connection_count, client_connection_count,
		upload_bytes_per_second, download_bytes_per_second, total_upload_bytes,
		total_download_bytes, reported_at
		FROM tunnel_runtime_status WHERE tunnel_id = ? LIMIT 1`, tunnelID).Scan(
		&status.HostConnectionCount, &status.ClientConnectionCount, &status.UploadBytesPerSecond,
		&status.DownloadBytesPerSecond, &status.TotalUploadBytes, &status.TotalDownloadBytes, &status.ReportedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *Store) InsertPort(ctx context.Context, port *core.TunnelPort) error {
	result, err := s.exec.ExecContext(ctx, `INSERT INTO tunnel_port
		(tunnel_code, port, protocol, allow_anonymous) VALUES (?, ?, ?, ?)`,
		port.TunnelCode, port.Port, port.Protocol, port.AllowAnonymous)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	port.ID = uint64(id)
	return nil
}

func (s *Store) FindPort(ctx context.Context, tunnelCode uint64, port uint16) (core.TunnelPort, error) {
	return scanPort(s.exec.QueryRowContext(ctx, `SELECT _id, tunnel_code, port, protocol, allow_anonymous
		FROM tunnel_port WHERE tunnel_code = ? AND port = ? LIMIT 1`, tunnelCode, port))
}

func (s *Store) ListPorts(ctx context.Context, tunnelCode uint64) ([]core.TunnelPort, error) {
	rows, err := s.exec.QueryContext(ctx, `SELECT _id, tunnel_code, port, protocol, allow_anonymous
		FROM tunnel_port WHERE tunnel_code = ? ORDER BY port`, tunnelCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]core.TunnelPort, 0)
	for rows.Next() {
		port, err := scanPort(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, port)
	}
	return result, rows.Err()
}

func (s *Store) CountPorts(ctx context.Context, tunnelCode uint64) (uint64, error) {
	var count uint64
	err := s.exec.QueryRowContext(ctx, `SELECT COUNT(*) FROM tunnel_port WHERE tunnel_code = ?`, tunnelCode).Scan(&count)
	return count, err
}

func (s *Store) UpdatePort(ctx context.Context, port core.TunnelPort) error {
	result, err := s.exec.ExecContext(ctx, `UPDATE tunnel_port SET protocol = ?, allow_anonymous = ? WHERE _id = ?`,
		port.Protocol, port.AllowAnonymous, port.ID)
	if err != nil {
		return err
	}
	return requireOneRow(result, "update tunnel port")
}

func (s *Store) DeletePort(ctx context.Context, id uint64) error {
	result, err := s.exec.ExecContext(ctx, `DELETE FROM tunnel_port WHERE _id = ?`, id)
	if err != nil {
		return err
	}
	return requireOneRow(result, "delete tunnel port")
}

func scanTunnel(row scanner, withPortCount bool) (core.Tunnel, error) {
	var tunnel core.Tunnel
	var description sql.NullString
	fields := []any{&tunnel.ID, &tunnel.Name, &tunnel.TunnelID, &tunnel.TunnelCode, &tunnel.ClusterID,
		&tunnel.Expiration, &tunnel.ExpirationHours, &tunnel.Namespace, &tunnel.AccountID, &description,
		&tunnel.BandwidthUsed, &tunnel.URL, &tunnel.Type, &tunnel.CreatedAt, &tunnel.UpdatedAt}
	if withPortCount {
		fields = append(fields, &tunnel.PortCount)
	}
	if err := row.Scan(fields...); err != nil {
		return core.Tunnel{}, err
	}
	if description.Valid {
		tunnel.Description = &description.String
	}
	return tunnel, nil
}

func scanPort(row scanner) (core.TunnelPort, error) {
	var port core.TunnelPort
	var databasePort uint64
	err := row.Scan(&port.ID, &port.TunnelCode, &databasePort, &port.Protocol, &port.AllowAnonymous)
	if err != nil {
		return core.TunnelPort{}, err
	}
	if databasePort > 65535 {
		return core.TunnelPort{}, fmt.Errorf("invalid port in database: %d", databasePort)
	}
	port.Port = uint16(databasePort)
	return port, nil
}

func requireOneRow(result sql.Result, operation string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("%s affected %d rows", operation, rows)
	}
	return nil
}

func placeholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}
