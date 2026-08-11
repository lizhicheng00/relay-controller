package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lizhicheng00/relay-controller/internal/config"
	"github.com/lizhicheng00/relay-controller/internal/core"
	"github.com/lizhicheng00/relay-controller/internal/security"
	"github.com/lizhicheng00/relay-controller/internal/store"
)

const tunnelCodeRetries = 5

type Service struct {
	store  *store.Store
	signer *security.JWTSigner
	config config.Relay
	log    *slog.Logger
	now    func() time.Time
}

func New(store *store.Store, signer *security.JWTSigner, cfg config.Relay, logger *slog.Logger) *Service {
	return &Service{store: store, signer: signer, config: cfg, log: logger, now: time.Now}
}

func (s *Service) CreateTunnel(ctx context.Context, namespace, accountNamespace string, request core.CreateTunnelRequest) (core.TunnelResponse, error) {
	namespace, accountNamespace, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return core.TunnelResponse{}, err
	}
	tunnelType, err := validateCreateTunnel(request)
	if err != nil {
		return core.TunnelResponse{}, err
	}
	if err := s.requireLocalCluster(ctx, request.ClusterID); err != nil {
		return core.TunnelResponse{}, err
	}
	if err := s.store.CreateAccountIfAbsent(ctx, accountNamespace, s.config.DefaultPlanCode); err != nil {
		return core.TunnelResponse{}, internal("create billing account", err)
	}
	now := s.now().Unix()
	expirationHours, expiration, err := core.ResolveExpiration(request.Expiration, s.config.DefaultExpirationHours, now)
	if err != nil {
		return core.TunnelResponse{}, err
	}

	var tunnel core.Tunnel
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		account, plan, err := s.lockActiveAccountPlan(ctx, tx, accountNamespace)
		if err != nil {
			return err
		}
		activeTunnels, err := tx.CountActiveTunnels(ctx, account.ID, now)
		if err != nil {
			return internal("count active tunnels", err)
		}
		if plan.MaxTunnels > 0 && activeTunnels >= uint64(plan.MaxTunnels) {
			return core.NewError(http.StatusTooManyRequests, core.CodeTunnelQuotaExceeded,
				fmt.Sprintf("active tunnel quota exceeded: max=%d", plan.MaxTunnels))
		}

		for attempt := 0; attempt < tunnelCodeRetries; attempt++ {
			code, tunnelID, err := core.NewTunnelCode()
			if err != nil {
				return internal("generate tunnel code", err)
			}
			tunnel = core.Tunnel{
				Name: request.Name, TunnelID: tunnelID, TunnelCode: code, ClusterID: request.ClusterID,
				Expiration: expiration, ExpirationHours: expirationHours, Namespace: namespace,
				AccountID: account.ID, Description: request.Description, URL: buildTunnelURL(tunnelID, request.ClusterID, s.config.Domain),
				Type: tunnelType, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.InsertTunnel(ctx, &tunnel); err == nil {
				return nil
			} else if !store.IsDuplicate(err) {
				return internal("insert tunnel", err)
			}
		}
		return core.NewError(http.StatusConflict, core.CodeTunnelIDConflict, "tunnel id conflict")
	})
	if err != nil {
		return core.TunnelResponse{}, err
	}
	s.log.Info("tunnel created", "tunnelId", tunnel.TunnelID, "namespace", tunnel.Namespace, "clusterId", tunnel.ClusterID)
	return tunnel.Response(nil), nil
}

func (s *Service) ListTunnels(ctx context.Context, namespace, accountNamespace, clusterID string) ([]core.TunnelListItem, error) {
	namespace, _, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return nil, err
	}
	if clusterID != "" {
		if err := s.requireLocalCluster(ctx, clusterID); err != nil {
			return nil, err
		}
	}
	tunnels, err := s.store.ListActiveTunnels(ctx, namespace, clusterID, s.config.Region, s.now().Unix())
	if err != nil {
		return nil, internal("list tunnels", err)
	}
	response := make([]core.TunnelListItem, 0, len(tunnels))
	for _, tunnel := range tunnels {
		response = append(response, tunnel.ListItem())
	}
	return response, nil
}

func (s *Service) GetTunnel(ctx context.Context, namespace, accountNamespace, tunnelID string) (core.TunnelResponse, error) {
	namespace, _, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return core.TunnelResponse{}, err
	}
	tunnel, err := s.ownedTunnel(ctx, s.store, namespace, tunnelID, true)
	if err != nil {
		return core.TunnelResponse{}, err
	}
	status, err := s.store.FindTunnelStatus(ctx, tunnelID)
	if err != nil {
		return core.TunnelResponse{}, internal("find tunnel status", err)
	}
	return tunnel.Response(status), nil
}

func (s *Service) UpdateTunnel(ctx context.Context, namespace, accountNamespace, tunnelID string, request core.UpdateTunnelRequest) (bool, error) {
	namespace, _, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return false, err
	}
	if err := validateUpdateTunnel(request); err != nil {
		return false, err
	}
	now := s.now().Unix()
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		tunnel, err := s.ownedTunnel(ctx, tx, namespace, tunnelID, true)
		if err != nil {
			return err
		}
		if request.Name != nil && strings.TrimSpace(*request.Name) != "" {
			tunnel.Name = *request.Name
		}
		if request.Description != nil {
			tunnel.Description = request.Description
		}
		if request.Type != nil {
			tunnel.Type, err = core.NormalizeTunnelType(request.Type, false)
			if err != nil {
				return err
			}
		}
		hours := tunnel.ExpirationHours
		if request.Expiration != nil {
			hours = *request.Expiration
		}
		tunnel.ExpirationHours, tunnel.Expiration, err = core.ResolveExpiration(&hours, s.config.DefaultExpirationHours, now)
		if err != nil {
			return err
		}
		tunnel.UpdatedAt = now
		if err := tx.UpdateTunnel(ctx, tunnel); err != nil {
			return internal("update tunnel", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	s.log.Info("tunnel updated", "tunnelId", tunnelID, "namespace", namespace)
	return true, nil
}

func (s *Service) DeleteTunnel(ctx context.Context, namespace, accountNamespace, tunnelID string) (bool, error) {
	namespace, _, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return false, err
	}
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		tunnel, err := s.ownedTunnel(ctx, tx, namespace, tunnelID, false)
		if err != nil {
			return err
		}
		if err := tx.DeleteTunnelGraph(ctx, tunnel); err != nil {
			return internal("delete tunnel", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	s.log.Info("tunnel deleted", "tunnelId", tunnelID, "namespace", namespace)
	return true, nil
}

func (s *Service) DeleteTunnels(ctx context.Context, namespace, accountNamespace string) (bool, error) {
	namespace, _, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return false, err
	}
	deleted := 0
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		tunnels, err := tx.LockNamespaceTunnels(ctx, namespace, s.config.Region)
		if err != nil {
			return internal("lock namespace tunnels", err)
		}
		for _, tunnel := range tunnels {
			if err := tx.DeleteTunnelGraph(ctx, tunnel); err != nil {
				return internal("delete namespace tunnel", err)
			}
			deleted++
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	s.log.Info("namespace tunnels deleted", "namespace", namespace, "count", deleted)
	return true, nil
}

func (s *Service) IssueTunnelToken(ctx context.Context, namespace, accountNamespace, tunnelID, scope string) (core.TunnelTokenResponse, error) {
	namespace, _, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return core.TunnelTokenResponse{}, err
	}
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope != "host" && scope != "connect" {
		return core.TunnelTokenResponse{}, core.Invalid("scope must be host or connect")
	}
	tunnel, err := s.ownedTunnel(ctx, s.store, namespace, tunnelID, true)
	if err != nil {
		return core.TunnelTokenResponse{}, err
	}
	if err := s.assertTrafficAllowed(ctx, tunnel.AccountID); err != nil {
		return core.TunnelTokenResponse{}, err
	}
	return s.signer.Issue(tunnel, scope, s.now().Unix())
}

func (s *Service) CreatePort(ctx context.Context, namespace, accountNamespace, tunnelID string, request core.CreateTunnelPortRequest) (core.TunnelPortResponse, error) {
	namespace, _, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	port, protocol, allowAnonymous, err := validateCreatePort(request)
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	var tunnel core.Tunnel
	tunnelPort := core.TunnelPort{Port: port, Protocol: protocol, AllowAnonymous: allowAnonymous}
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		tunnel, err = s.ownedTunnel(ctx, tx, namespace, tunnelID, true)
		if err != nil {
			return err
		}
		tunnelPort.TunnelCode = tunnel.TunnelCode
		if _, err := tx.FindPort(ctx, tunnel.TunnelCode, port); err == nil {
			return core.NewError(http.StatusConflict, core.CodeTunnelPortExists, "tunnel port already exists")
		} else if !errors.Is(err, sql.ErrNoRows) {
			return internal("find tunnel port", err)
		}
		_, plan, err := s.lockActiveAccountPlanByID(ctx, tx, tunnel.AccountID)
		if err != nil {
			return err
		}
		count, err := tx.CountPorts(ctx, tunnel.TunnelCode)
		if err != nil {
			return internal("count tunnel ports", err)
		}
		if plan.MaxPortsPerTunnel > 0 && count >= uint64(plan.MaxPortsPerTunnel) {
			return core.NewError(http.StatusTooManyRequests, core.CodeTunnelPortQuotaExceeded,
				fmt.Sprintf("tunnel port quota exceeded: max=%d", plan.MaxPortsPerTunnel))
		}
		if err := tx.InsertPort(ctx, &tunnelPort); err != nil {
			if store.IsDuplicate(err) {
				return core.NewError(http.StatusConflict, core.CodeTunnelPortExists, "tunnel port already exists")
			}
			return internal("insert tunnel port", err)
		}
		if err := tx.RefreshTunnelExpiration(ctx, tunnelID, s.config.Region, s.now().Unix()); err != nil {
			return internal("refresh tunnel expiration", err)
		}
		return nil
	})
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	s.log.Info("tunnel port created", "tunnelId", tunnelID, "port", port, "protocol", protocol)
	return core.PortResponse(tunnel, tunnelPort), nil
}

func (s *Service) ListPorts(ctx context.Context, namespace, accountNamespace, tunnelID string) ([]core.TunnelPortResponse, error) {
	namespace, _, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return nil, err
	}
	tunnel, err := s.ownedTunnel(ctx, s.store, namespace, tunnelID, true)
	if err != nil {
		return nil, err
	}
	ports, err := s.store.ListPorts(ctx, tunnel.TunnelCode)
	if err != nil {
		return nil, internal("list tunnel ports", err)
	}
	response := make([]core.TunnelPortResponse, 0, len(ports))
	for _, port := range ports {
		response = append(response, core.PortResponse(tunnel, port))
	}
	return response, nil
}

func (s *Service) GetPort(ctx context.Context, namespace, accountNamespace, tunnelID string, port uint16) (core.TunnelPortResponse, error) {
	namespace, _, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	tunnel, err := s.ownedTunnel(ctx, s.store, namespace, tunnelID, true)
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	tunnelPort, err := s.store.FindPort(ctx, tunnel.TunnelCode, port)
	if errors.Is(err, sql.ErrNoRows) {
		return core.TunnelPortResponse{}, core.NewError(http.StatusNotFound, core.CodeTunnelPortNotFound, "tunnel port not found")
	}
	if err != nil {
		return core.TunnelPortResponse{}, internal("find tunnel port", err)
	}
	return core.PortResponse(tunnel, tunnelPort), nil
}

func (s *Service) UpdatePort(ctx context.Context, namespace, accountNamespace, tunnelID string, port uint16, request core.UpdateTunnelPortRequest) (core.TunnelPortResponse, error) {
	namespace, _, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	protocol, err := core.NormalizeProtocol(request.Protocol, false)
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	var tunnel core.Tunnel
	var tunnelPort core.TunnelPort
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		tunnel, err = s.ownedTunnel(ctx, tx, namespace, tunnelID, true)
		if err != nil {
			return err
		}
		tunnelPort, err = tx.FindPort(ctx, tunnel.TunnelCode, port)
		if errors.Is(err, sql.ErrNoRows) {
			return core.NewError(http.StatusNotFound, core.CodeTunnelPortNotFound, "tunnel port not found")
		}
		if err != nil {
			return internal("find tunnel port", err)
		}
		if request.Protocol == nil && request.AllowAnonymous == nil {
			return nil
		}
		if request.Protocol != nil {
			tunnelPort.Protocol = protocol
		}
		if request.AllowAnonymous != nil {
			tunnelPort.AllowAnonymous = *request.AllowAnonymous
		}
		if err := tx.UpdatePort(ctx, tunnelPort); err != nil {
			return internal("update tunnel port", err)
		}
		if err := tx.RefreshTunnelExpiration(ctx, tunnelID, s.config.Region, s.now().Unix()); err != nil {
			return internal("refresh tunnel expiration", err)
		}
		return nil
	})
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	return core.PortResponse(tunnel, tunnelPort), nil
}

func (s *Service) DeletePort(ctx context.Context, namespace, accountNamespace, tunnelID string, port uint16) (bool, error) {
	namespace, _, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return false, err
	}
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		tunnel, err := s.ownedTunnel(ctx, tx, namespace, tunnelID, true)
		if err != nil {
			return err
		}
		tunnelPort, err := tx.FindPort(ctx, tunnel.TunnelCode, port)
		if errors.Is(err, sql.ErrNoRows) {
			return core.NewError(http.StatusNotFound, core.CodeTunnelPortNotFound, "tunnel port not found")
		}
		if err != nil {
			return internal("find tunnel port", err)
		}
		if err := tx.DeletePort(ctx, tunnelPort.ID); err != nil {
			return internal("delete tunnel port", err)
		}
		if err := tx.RefreshTunnelExpiration(ctx, tunnelID, s.config.Region, s.now().Unix()); err != nil {
			return internal("refresh tunnel expiration", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	s.log.Info("tunnel port deleted", "tunnelId", tunnelID, "port", port)
	return true, nil
}

func (s *Service) GetLimits(ctx context.Context, namespace, accountNamespace string) (core.LimitsResponse, error) {
	_, accountNamespace, err := requireContext(namespace, accountNamespace)
	if err != nil {
		return core.LimitsResponse{}, err
	}
	now := s.now().Unix()
	var response core.LimitsResponse
	if err := s.store.CreateAccountIfAbsent(ctx, accountNamespace, s.config.DefaultPlanCode); err != nil {
		return core.LimitsResponse{}, internal("create billing account", err)
	}
	err = s.store.InTx(ctx, func(tx *store.Store) error {
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

func (s *Service) requireLocalCluster(ctx context.Context, clusterID string) error {
	if !core.ValidIdentifier(clusterID) {
		return core.Invalid("clusterId is invalid")
	}
	exists, err := s.store.ClusterExists(ctx, clusterID, s.config.Region)
	if err != nil {
		return internal("find local cluster", err)
	}
	if !exists {
		return core.NewError(http.StatusNotFound, core.CodeClusterNotFound, "cluster not found")
	}
	return nil
}

func (s *Service) ownedTunnel(ctx context.Context, database *store.Store, namespace, tunnelID string, requireActive bool) (core.Tunnel, error) {
	var tunnel core.Tunnel
	var err error
	if database == s.store {
		tunnel, err = database.FindTunnel(ctx, tunnelID, s.config.Region)
	} else {
		tunnel, err = database.LockTunnel(ctx, tunnelID, s.config.Region)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return core.Tunnel{}, core.NewError(http.StatusNotFound, core.CodeTunnelNotFound, "tunnel not found")
	}
	if err != nil {
		return core.Tunnel{}, internal("find tunnel", err)
	}
	if tunnel.Namespace != namespace {
		return core.Tunnel{}, core.NewError(http.StatusForbidden, core.CodeTunnelAccessDenied, "tunnel access denied")
	}
	if requireActive && tunnel.Expiration <= s.now().Unix() {
		return core.Tunnel{}, core.NewError(http.StatusGone, core.CodeTunnelExpired, "tunnel expired")
	}
	return tunnel, nil
}

func (s *Service) lockActiveAccountPlan(ctx context.Context, tx *store.Store, namespace string) (core.BillingAccount, core.BillingPlan, error) {
	account, err := tx.LockAccountByNamespace(ctx, namespace)
	if err != nil {
		return core.BillingAccount{}, core.BillingPlan{}, internal("lock billing account", err)
	}
	return s.activeAccountPlan(ctx, tx, account)
}

func (s *Service) lockActiveAccountPlanByID(ctx context.Context, tx *store.Store, accountID uint64) (core.BillingAccount, core.BillingPlan, error) {
	account, err := tx.LockAccountByID(ctx, accountID)
	if err != nil {
		return core.BillingAccount{}, core.BillingPlan{}, internal("lock billing account", err)
	}
	return s.activeAccountPlan(ctx, tx, account)
}

func (s *Service) activeAccountPlan(ctx context.Context, tx *store.Store, account core.BillingAccount) (core.BillingAccount, core.BillingPlan, error) {
	if account.Status != "active" {
		return core.BillingAccount{}, core.BillingPlan{}, core.NewError(http.StatusForbidden, core.CodeAccountDisabled, "account disabled")
	}
	plan, err := s.requirePlan(ctx, tx, account.PlanCode)
	return account, plan, err
}

func (s *Service) requirePlan(ctx context.Context, database *store.Store, planCode string) (core.BillingPlan, error) {
	plan, err := database.FindPlan(ctx, planCode)
	if errors.Is(err, sql.ErrNoRows) {
		return core.BillingPlan{}, internal("billing plan unavailable", err)
	}
	if err != nil {
		return core.BillingPlan{}, internal("find billing plan", err)
	}
	return plan, nil
}

func (s *Service) assertTrafficAllowed(ctx context.Context, accountID uint64) error {
	return s.store.InTx(ctx, func(tx *store.Store) error {
		account, plan, err := s.lockActiveAccountPlanByID(ctx, tx, accountID)
		if err != nil {
			return err
		}
		period, err := ensurePeriod(ctx, tx, account.ID, plan, s.now().Unix())
		if err != nil {
			return err
		}
		if s.config.BillingEnforcement && period.BilledBytes >= period.QuotaBytes {
			return core.NewError(http.StatusTooManyRequests, core.CodeAccountQuotaExceeded, "monthly traffic quota exceeded")
		}
		return nil
	})
}

func ensurePeriod(ctx context.Context, tx *store.Store, accountID uint64, plan core.BillingPlan, timestamp int64) (core.BillingPeriod, error) {
	start, end := core.BillingPeriodRange(timestamp)
	if err := tx.CreatePeriodIfAbsent(ctx, accountID, start, end, plan.MonthlyQuotaBytes); err != nil {
		return core.BillingPeriod{}, internal("create billing period", err)
	}
	period, err := tx.FindPeriod(ctx, accountID, start)
	if err != nil {
		return core.BillingPeriod{}, internal("find billing period", err)
	}
	return period, nil
}

func requireContext(namespace, accountNamespace string) (string, string, error) {
	if strings.TrimSpace(namespace) == "" {
		return "", "", core.MissingHeader("X-Namespace")
	}
	if !core.ValidIdentifier(namespace) {
		return "", "", core.Invalid("X-Namespace is invalid")
	}
	if strings.TrimSpace(accountNamespace) == "" {
		return "", "", core.MissingHeader("X-Account-Namespace")
	}
	if !core.ValidIdentifier(accountNamespace) {
		return "", "", core.Invalid("X-Account-Namespace is invalid")
	}
	return namespace, accountNamespace, nil
}

func validateCreateTunnel(request core.CreateTunnelRequest) (string, error) {
	if strings.TrimSpace(request.Name) == "" {
		return "", core.InvalidField("name", "must not be blank")
	}
	if utf8.RuneCountInString(request.Name) > 128 {
		return "", core.InvalidField("name", "must not exceed 128 characters")
	}
	if request.Description != nil && utf8.RuneCountInString(*request.Description) > 512 {
		return "", core.InvalidField("description", "must not exceed 512 characters")
	}
	if !core.ValidIdentifier(request.ClusterID) {
		return "", core.InvalidField("clusterId", "is invalid")
	}
	if request.Expiration != nil && (*request.Expiration < 1 || *request.Expiration > 720) {
		return "", core.InvalidField("expiration", "must be between 1 and 720")
	}
	return core.NormalizeTunnelType(request.Type, true)
}

func validateUpdateTunnel(request core.UpdateTunnelRequest) error {
	if request.Name != nil && utf8.RuneCountInString(*request.Name) > 128 {
		return core.InvalidField("name", "must not exceed 128 characters")
	}
	if request.Description != nil && utf8.RuneCountInString(*request.Description) > 512 {
		return core.InvalidField("description", "must not exceed 512 characters")
	}
	if request.Expiration != nil && (*request.Expiration < 1 || *request.Expiration > 720) {
		return core.InvalidField("expiration", "must be between 1 and 720")
	}
	if request.Type != nil {
		_, err := core.NormalizeTunnelType(request.Type, false)
		return err
	}
	return nil
}

func validateCreatePort(request core.CreateTunnelPortRequest) (uint16, string, bool, error) {
	if request.Port == nil || *request.Port < 1 || *request.Port > 65535 {
		return 0, "", false, core.NewError(http.StatusBadRequest, core.CodeTunnelPortInvalid, "tunnel port invalid")
	}
	protocol, err := core.NormalizeProtocol(request.Protocol, true)
	if err != nil {
		return 0, "", false, err
	}
	if request.AllowAnonymous == nil {
		return 0, "", false, core.InvalidField("allowAnonymous", "is required")
	}
	return uint16(*request.Port), protocol, *request.AllowAnonymous, nil
}

func buildTunnelURL(tunnelID, clusterID, domain string) string {
	return tunnelID + "." + clusterID + "." + domain
}

func internal(operation string, err error) error {
	return core.Internal(fmt.Errorf("%s: %w", operation, err))
}
