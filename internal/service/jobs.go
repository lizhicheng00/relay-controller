package service

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/lizhicheng00/relay-controller/internal/core"
	"github.com/lizhicheng00/relay-controller/internal/store"
)

const (
	cleanupBatchSize = 100
	partitionLock    = "meteringPartitions"
	hourSeconds      = int64(3600)
	retentionSeconds = 7 * 24 * hourSeconds
	futureHours      = 2
)

type periodKey struct {
	accountID uint64
	start     int64
}

type tunnelUsage struct {
	bytes          uint64
	lastReportedAt int64
}

func (s *Service) StartJobs(ctx context.Context) *sync.WaitGroup {
	var waitGroup sync.WaitGroup
	if s.config.BillingSettlement {
		startJob(ctx, &waitGroup, s.config.SettlementInterval, s.config.SettlementInterval, s.runSettlement)
	}
	startJob(ctx, &waitGroup, s.config.CleanupInitialDelay, s.config.CleanupInterval, s.runCleanup)
	startJob(ctx, &waitGroup, s.config.PartitionInitialDelay, s.config.PartitionInterval, s.runPartitionMaintenance)
	return &waitGroup
}

func (s *Service) SettleBatch(ctx context.Context, limit int) (int, error) {
	clusterIDs, err := s.store.LocalClusterIDs(ctx, s.config.Region)
	if err != nil {
		return 0, internal("list local clusters", err)
	}
	if len(clusterIDs) == 0 {
		return 0, nil
	}
	limit = max(1, limit)
	settled := 0
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		records, err := tx.LockUnsettledMetering(ctx, clusterIDs, limit)
		if err != nil {
			return internal("lock unsettled metering", err)
		}
		if len(records) == 0 {
			return nil
		}

		usageByPeriod := make(map[periodKey]uint64)
		usageByTunnel := make(map[string]tunnelUsage)
		for _, record := range records {
			usage, err := core.AddBytes(record.UploadBytes, record.DownloadBytes)
			if err != nil {
				return internal("sum metering directions", err)
			}
			if usage == 0 {
				continue
			}
			periodStart, _ := core.BillingPeriodRange(record.ReportedAt)
			key := periodKey{accountID: record.AccountID, start: periodStart}
			usageByPeriod[key], err = core.AddBytes(usageByPeriod[key], usage)
			if err != nil {
				return internal("sum account metering", err)
			}
			current := usageByTunnel[record.TunnelID]
			current.bytes, err = core.AddBytes(current.bytes, usage)
			if err != nil {
				return internal("sum tunnel metering", err)
			}
			current.lastReportedAt = max(current.lastReportedAt, record.ReportedAt)
			usageByTunnel[record.TunnelID] = current
		}

		tunnelIDs := make([]string, 0, len(usageByTunnel))
		for tunnelID := range usageByTunnel {
			tunnelIDs = append(tunnelIDs, tunnelID)
		}
		sort.Strings(tunnelIDs)
		settledAt := uint64(s.now().Unix())
		for _, tunnelID := range tunnelIDs {
			usage := usageByTunnel[tunnelID]
			if err := tx.IncreaseTunnelUsage(ctx, tunnelID, s.config.Region, usage.bytes, settledAt); err != nil {
				return internal("increase tunnel usage", err)
			}
			if err := tx.RefreshTunnelExpiration(ctx, tunnelID, s.config.Region, usage.lastReportedAt); err != nil {
				return internal("refresh tunnel expiration", err)
			}
		}

		periodKeys := make([]periodKey, 0, len(usageByPeriod))
		for key := range usageByPeriod {
			periodKeys = append(periodKeys, key)
		}
		sort.Slice(periodKeys, func(first, second int) bool {
			if periodKeys[first].accountID == periodKeys[second].accountID {
				return periodKeys[first].start < periodKeys[second].start
			}
			return periodKeys[first].accountID < periodKeys[second].accountID
		})
		for _, key := range periodKeys {
			account, err := tx.LockAccountByID(ctx, key.accountID)
			if err != nil {
				return internal("lock metering account", err)
			}
			plan, err := s.requirePlan(ctx, tx, account.PlanCode)
			if err != nil {
				return err
			}
			if _, err := ensurePeriod(ctx, tx, account.ID, plan, key.start); err != nil {
				return err
			}
			if err := tx.IncreasePeriodUsage(ctx, account.ID, key.start, usageByPeriod[key]); err != nil {
				return internal("increase billing usage", err)
			}
		}
		for _, key := range periodKeys {
			if err := tx.BlockQuotaIfExhausted(ctx, key.accountID, key.start); err != nil {
				return internal("update quota block", err)
			}
		}
		marked, err := tx.MarkMeteringSettled(ctx, records)
		if err != nil {
			return internal("mark metering settled", err)
		}
		if marked != int64(len(records)) {
			return internal("mark metering settled", fmt.Errorf("marked %d of %d records", marked, len(records)))
		}
		settled = len(records)
		return nil
	})
	return settled, err
}

func (s *Service) CleanupAgedTunnels(ctx context.Context, now int64) (int, error) {
	cutoff := now - int64(s.config.CleanupRetentionDays)*24*hourSeconds
	deleted := 0
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		tunnels, err := tx.LockAgedTunnels(ctx, s.config.Region, cutoff, cleanupBatchSize)
		if err != nil {
			return internal("lock aged tunnels", err)
		}
		for _, tunnel := range tunnels {
			if err := tx.DeleteTunnelGraph(ctx, tunnel); err != nil {
				return internal("delete aged tunnel", err)
			}
			deleted++
		}
		return nil
	})
	return deleted, err
}

func (s *Service) MaintainPartitions(ctx context.Context, now int64) error {
	owner := schedulerOwner()
	locked, err := s.store.TrySchedulerLock(ctx, partitionLock, owner, 30*time.Minute)
	if err != nil {
		return internal("acquire partition lock", err)
	}
	if !locked {
		return nil
	}
	defer func() {
		if err := s.store.ReleaseSchedulerLock(context.Background(), partitionLock, owner); err != nil {
			s.log.Error("failed to release partition lock", "error", err)
		}
	}()

	partitions, err := s.store.MeteringPartitions(ctx)
	if err != nil {
		return internal("load metering partitions", err)
	}
	currentHour := now - now%hourSeconds
	var latest *int64
	if len(partitions) > 0 {
		boundary := partitions[len(partitions)-1].Boundary
		latest = &boundary
	}
	boundaries := partitionBoundaries(latest, currentHour)
	definitions := make([]string, 0, len(boundaries))
	for _, boundary := range boundaries {
		definitions = append(definitions, fmt.Sprintf("PARTITION %s VALUES LESS THAN (%d)", partitionName(boundary), boundary))
	}
	if err := s.store.ReorganizeFuturePartition(ctx, definitions); err != nil {
		return internal("create metering partitions", err)
	}
	cutoff := currentHour - retentionSeconds
	var expired []string
	for _, partition := range partitions {
		if partition.Boundary <= cutoff {
			expired = append(expired, partition.Name)
		}
	}
	if err := s.store.DropMeteringPartitions(ctx, expired); err != nil {
		return internal("drop metering partitions", err)
	}
	return nil
}

func (s *Service) runSettlement(ctx context.Context) {
	settled := 0
	for {
		count, err := s.SettleBatch(ctx, s.config.SettlementBatchSize)
		if err != nil {
			s.log.Error("billing settlement failed", "error", err)
			return
		}
		settled += count
		if count < s.config.SettlementBatchSize {
			break
		}
	}
	if settled > 0 {
		s.log.Info("billing settlement completed", "records", settled)
	}
}

func (s *Service) runCleanup(ctx context.Context) {
	deleted, err := s.CleanupAgedTunnels(ctx, s.now().Unix())
	if err != nil {
		s.log.Error("tunnel cleanup failed", "error", err)
		return
	}
	if deleted > 0 {
		s.log.Info("aged tunnels deleted", "region", s.config.Region, "count", deleted)
	}
}

func (s *Service) runPartitionMaintenance(ctx context.Context) {
	if err := s.MaintainPartitions(ctx, s.now().Unix()); err != nil {
		s.log.Error("metering partition maintenance failed", "error", err)
	}
}

func startJob(ctx context.Context, waitGroup *sync.WaitGroup, initialDelay, interval time.Duration, job func(context.Context)) {
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		initial := time.NewTimer(initialDelay)
		defer initial.Stop()
		select {
		case <-ctx.Done():
			return
		case <-initial.C:
		}
		job(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				job(ctx)
			}
		}
	}()
}

func partitionBoundaries(latest *int64, currentHour int64) []int64 {
	firstRetained := currentHour - retentionSeconds + hourSeconds
	next := firstRetained
	if latest != nil {
		next = max(*latest+hourSeconds, firstRetained)
	}
	final := currentHour + (futureHours+1)*hourSeconds
	var boundaries []int64
	for boundary := next; boundary <= final; boundary += hourSeconds {
		boundaries = append(boundaries, boundary)
	}
	return boundaries
}

func partitionName(boundary int64) string {
	return "p_" + time.Unix(boundary-hourSeconds, 0).UTC().Format("2006010215")
}

func schedulerOwner() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "relay-controller"
	}
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}
