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

	"relay-controller/internal/core"
	"relay-controller/internal/security"
	"relay-controller/internal/store"
)

const (
	tunnelCodeRetries      = 5
	defaultExpirationHours = 72
	defaultPlanCode        = "trial"
)

type Service struct {
	store  *store.Store
	signer *security.JWTSigner
	domain string
	region string
	log    *slog.Logger
	now    func() time.Time
}

func New(store *store.Store, signer *security.JWTSigner, domain, region string, logger *slog.Logger) *Service {
	return &Service{store: store, signer: signer, domain: domain, region: region, log: logger, now: time.Now}
}

func (s *Service) CreateTunnel(ctx context.Context, namespace, accountNamespace string, request core.CreateTunnelRequest) (core.TunnelResponse, error) {
	tunnelType, err := validateCreateTunnel(request)
	if err != nil {
		return core.TunnelResponse{}, err
	}
	now := s.now().Unix()
	expirationHours := defaultExpirationHours
	if request.Expiration != nil {
		expirationHours = *request.Expiration
	}
	expiration, err := core.ExpirationAt(expirationHours, now)
	if err != nil {
		return core.TunnelResponse{}, err
	}
	if err := s.requireLocalCluster(ctx, request.ClusterID); err != nil {
		return core.TunnelResponse{}, err
	}
	if err := s.store.CreateAccountIfAbsent(ctx, accountNamespace, defaultPlanCode); err != nil {
		return core.TunnelResponse{}, internal("create billing account", err)
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
		if activeTunnels >= uint64(plan.MaxTunnels) {
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
				AccountID: account.ID, Description: request.Description, URL: buildTunnelURL(tunnelID, request.ClusterID, s.domain),
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

func (s *Service) ListTunnels(ctx context.Context, namespace, clusterID string) ([]core.TunnelListItem, error) {
	if clusterID != "" {
		if err := s.requireLocalCluster(ctx, clusterID); err != nil {
			return nil, err
		}
	}
	tunnels, err := s.store.ListActiveTunnels(ctx, namespace, clusterID, s.region, s.now().Unix())
	if err != nil {
		return nil, internal("list tunnels", err)
	}
	response := make([]core.TunnelListItem, 0, len(tunnels))
	for _, tunnel := range tunnels {
		response = append(response, tunnel.ListItem())
	}
	return response, nil
}

func (s *Service) GetTunnel(ctx context.Context, namespace, tunnelID string) (core.TunnelResponse, error) {
	tunnel, err := s.findOwnedTunnel(ctx, namespace, tunnelID, true)
	if err != nil {
		return core.TunnelResponse{}, err
	}
	status, err := s.store.FindTunnelStatus(ctx, tunnelID)
	if err != nil {
		return core.TunnelResponse{}, internal("find tunnel status", err)
	}
	return tunnel.Response(status), nil
}

func (s *Service) UpdateTunnel(ctx context.Context, namespace, tunnelID string, request core.UpdateTunnelRequest) (bool, error) {
	if err := validateUpdateTunnel(request); err != nil {
		return false, err
	}
	var tunnelType string
	if request.Type != nil {
		var err error
		tunnelType, err = core.NormalizeTunnelType(*request.Type)
		if err != nil {
			return false, err
		}
	}
	now := s.now().Unix()
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		tunnel, err := s.lockOwnedTunnel(ctx, tx, namespace, tunnelID, true)
		if err != nil {
			return err
		}
		if request.Name != nil {
			tunnel.Name = *request.Name
		}
		if request.Description != nil {
			tunnel.Description = request.Description
		}
		if request.Type != nil {
			tunnel.Type = tunnelType
		}
		if request.Expiration != nil {
			tunnel.ExpirationHours = *request.Expiration
			tunnel.Expiration, err = core.ExpirationAt(*request.Expiration, now)
			if err != nil {
				return err
			}
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

func (s *Service) DeleteTunnel(ctx context.Context, namespace, tunnelID string) (bool, error) {
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		tunnel, err := s.lockOwnedTunnel(ctx, tx, namespace, tunnelID, false)
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

func (s *Service) DeleteTunnels(ctx context.Context, namespace string) (bool, error) {
	deleted := 0
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		tunnels, err := tx.LockNamespaceTunnels(ctx, namespace, s.region)
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

func (s *Service) IssueTunnelToken(ctx context.Context, namespace, tunnelID, scope string) (core.TunnelTokenResponse, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope != "host" && scope != "connect" {
		return core.TunnelTokenResponse{}, core.InvalidField("scope", "must be host or connect")
	}
	tunnel, err := s.findOwnedTunnel(ctx, namespace, tunnelID, true)
	if err != nil {
		return core.TunnelTokenResponse{}, err
	}
	if err := s.assertTrafficAllowed(ctx, tunnel.AccountID); err != nil {
		return core.TunnelTokenResponse{}, err
	}
	return s.signer.Issue(tunnel, scope, s.now().Unix())
}

func (s *Service) requireLocalCluster(ctx context.Context, clusterID string) error {
	if !core.ValidIdentifier(clusterID) {
		return core.InvalidField("clusterId", "is invalid")
	}
	exists, err := s.store.ClusterExists(ctx, clusterID, s.region)
	if err != nil {
		return internal("find local cluster", err)
	}
	if !exists {
		return core.NewError(http.StatusNotFound, core.CodeClusterNotFound, "cluster not found")
	}
	return nil
}

func (s *Service) findOwnedTunnel(ctx context.Context, namespace, tunnelID string, requireActive bool) (core.Tunnel, error) {
	tunnel, err := s.store.FindTunnel(ctx, tunnelID, s.region)
	return s.checkOwnedTunnel(tunnel, err, namespace, requireActive)
}

func (s *Service) lockOwnedTunnel(ctx context.Context, tx *store.Store, namespace, tunnelID string, requireActive bool) (core.Tunnel, error) {
	tunnel, err := tx.LockTunnel(ctx, tunnelID, s.region)
	return s.checkOwnedTunnel(tunnel, err, namespace, requireActive)
}

func (s *Service) checkOwnedTunnel(tunnel core.Tunnel, err error, namespace string, requireActive bool) (core.Tunnel, error) {
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
		if period.BilledBytes >= period.QuotaBytes {
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
	if request.Type == nil || strings.TrimSpace(*request.Type) == "" {
		return "bridge", nil
	}
	return core.NormalizeTunnelType(*request.Type)
}

func validateUpdateTunnel(request core.UpdateTunnelRequest) error {
	if request.Name != nil {
		if strings.TrimSpace(*request.Name) == "" {
			return core.InvalidField("name", "must not be blank")
		}
		if utf8.RuneCountInString(*request.Name) > 128 {
			return core.InvalidField("name", "must not exceed 128 characters")
		}
	}
	if request.Description != nil && utf8.RuneCountInString(*request.Description) > 512 {
		return core.InvalidField("description", "must not exceed 512 characters")
	}
	return nil
}

func buildTunnelURL(tunnelID, clusterID, domain string) string {
	return tunnelID + "." + clusterID + "." + domain
}

func internal(operation string, err error) error {
	return core.Internal(fmt.Errorf("%s: %w", operation, err))
}
