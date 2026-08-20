package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"relay-controller/internal/core"
	"relay-controller/internal/store"
)

func (s *Service) CreatePort(ctx context.Context, namespace, tunnelID string, request core.CreateTunnelPortRequest) (core.TunnelPortResponse, error) {
	port, protocol, allowAnonymous, err := validateCreatePort(request)
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	var tunnel core.Tunnel
	tunnelPort := core.TunnelPort{Port: port, Protocol: protocol, AllowAnonymous: allowAnonymous}
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		tunnel, err = s.lockOwnedTunnel(ctx, tx, namespace, tunnelID, true)
		if err != nil {
			return err
		}
		tunnelPort.TunnelCode = tunnel.TunnelCode
		_, plan, err := s.lockActiveAccountPlanByID(ctx, tx, tunnel.AccountID)
		if err != nil {
			return err
		}
		count, err := tx.CountPorts(ctx, tunnel.TunnelCode)
		if err != nil {
			return internal("count tunnel ports", err)
		}
		if count >= uint64(plan.MaxPortsPerTunnel) {
			return core.NewError(http.StatusTooManyRequests, core.CodeTunnelPortQuotaExceeded,
				fmt.Sprintf("tunnel port quota exceeded: max=%d", plan.MaxPortsPerTunnel))
		}
		if err := tx.InsertPort(ctx, &tunnelPort); err != nil {
			if store.IsDuplicate(err) {
				return core.NewError(http.StatusConflict, core.CodeTunnelPortExists, "tunnel port already exists")
			}
			return internal("insert tunnel port", err)
		}
		if err := tx.RefreshTunnelExpiration(ctx, tunnelID, s.region, s.now().Unix()); err != nil {
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

func (s *Service) ListPorts(ctx context.Context, namespace, tunnelID string) ([]core.TunnelPortResponse, error) {
	tunnel, err := s.findOwnedTunnel(ctx, namespace, tunnelID, true)
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

func (s *Service) GetPort(ctx context.Context, namespace, tunnelID string, port uint16) (core.TunnelPortResponse, error) {
	tunnel, err := s.findOwnedTunnel(ctx, namespace, tunnelID, true)
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	tunnelPort, err := findPort(ctx, s.store, tunnel.TunnelCode, port)
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	return core.PortResponse(tunnel, tunnelPort), nil
}

func (s *Service) UpdatePort(ctx context.Context, namespace, tunnelID string, port uint16, request core.UpdateTunnelPortRequest) (core.TunnelPortResponse, error) {
	var err error
	protocol := ""
	if request.Protocol != nil {
		protocol, err = core.NormalizeProtocol(*request.Protocol)
		if err != nil {
			return core.TunnelPortResponse{}, err
		}
	}
	var tunnel core.Tunnel
	var tunnelPort core.TunnelPort
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		tunnel, err = s.lockOwnedTunnel(ctx, tx, namespace, tunnelID, true)
		if err != nil {
			return err
		}
		tunnelPort, err = findPort(ctx, tx, tunnel.TunnelCode, port)
		if err != nil {
			return err
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
		if err := tx.RefreshTunnelExpiration(ctx, tunnelID, s.region, s.now().Unix()); err != nil {
			return internal("refresh tunnel expiration", err)
		}
		return nil
	})
	if err != nil {
		return core.TunnelPortResponse{}, err
	}
	return core.PortResponse(tunnel, tunnelPort), nil
}

func (s *Service) DeletePort(ctx context.Context, namespace, tunnelID string, port uint16) (bool, error) {
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		tunnel, err := s.lockOwnedTunnel(ctx, tx, namespace, tunnelID, true)
		if err != nil {
			return err
		}
		tunnelPort, err := findPort(ctx, tx, tunnel.TunnelCode, port)
		if err != nil {
			return err
		}
		if err := tx.DeletePort(ctx, tunnelPort.ID); err != nil {
			return internal("delete tunnel port", err)
		}
		if err := tx.RefreshTunnelExpiration(ctx, tunnelID, s.region, s.now().Unix()); err != nil {
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

func validateCreatePort(request core.CreateTunnelPortRequest) (uint16, string, bool, error) {
	if request.Port == nil || *request.Port < 1 || *request.Port > 65535 {
		return 0, "", false, core.NewError(http.StatusBadRequest, core.CodeTunnelPortInvalid, "tunnel port invalid")
	}
	if request.Protocol == nil {
		return 0, "", false, core.InvalidField("protocol", "is required")
	}
	protocol, err := core.NormalizeProtocol(*request.Protocol)
	if err != nil {
		return 0, "", false, err
	}
	if request.AllowAnonymous == nil {
		return 0, "", false, core.InvalidField("allowAnonymous", "is required")
	}
	return uint16(*request.Port), protocol, *request.AllowAnonymous, nil
}

func findPort(ctx context.Context, database *store.Store, tunnelCode uint64, port uint16) (core.TunnelPort, error) {
	tunnelPort, err := database.FindPort(ctx, tunnelCode, port)
	if errors.Is(err, sql.ErrNoRows) {
		return core.TunnelPort{}, core.NewError(http.StatusNotFound, core.CodeTunnelPortNotFound, "tunnel port not found")
	}
	if err != nil {
		return core.TunnelPort{}, internal("find tunnel port", err)
	}
	return tunnelPort, nil
}
