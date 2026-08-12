package service

import (
	"context"

	"github.com/lizhicheng00/relay-controller/relay-controller/internal/core"
	"github.com/lizhicheng00/relay-controller/relay-controller/internal/store"
)

func (s *Service) GetLimits(ctx context.Context, accountNamespace string) (core.LimitsResponse, error) {
	now := s.now().Unix()
	var response core.LimitsResponse
	if err := s.store.CreateAccountIfAbsent(ctx, accountNamespace, defaultPlanCode); err != nil {
		return core.LimitsResponse{}, internal("create billing account", err)
	}
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		account, err := tx.LockAccountByNamespace(ctx, accountNamespace)
		if err != nil {
			return internal("lock billing account", err)
		}
		plan, err := s.requirePlan(ctx, tx, account.PlanCode)
		if err != nil {
			return err
		}
		period, err := ensurePeriod(ctx, tx, account.ID, plan, now)
		if err != nil {
			return err
		}
		activeTunnels, err := tx.CountActiveTunnels(ctx, account.ID, now)
		if err != nil {
			return internal("count active tunnels", err)
		}
		remaining := uint64(0)
		if period.BilledBytes < period.QuotaBytes {
			remaining = period.QuotaBytes - period.BilledBytes
		}
		response = core.LimitsResponse{
			ResetAt: period.End, QuotaBytes: period.QuotaBytes, RemainingBytes: remaining,
			ActiveTunnels: activeTunnels, MaxTunnels: plan.MaxTunnels, MaxPortsPerTunnel: plan.MaxPortsPerTunnel,
			MaxHostsPerTunnel:                plan.MaxHostsPerTunnel,
			MaxTunnelBandwidthBytesPerSecond: plan.MaxTunnelBandwidthBytesPerSecond,
			MaxHTTPRequestsPerMinutePerPort:  plan.MaxHTTPRequestsPerMinutePerPort,
			MaxConnectionsPerPort:            plan.MaxConnectionsPerPort,
		}
		return nil
	})
	return response, err
}
