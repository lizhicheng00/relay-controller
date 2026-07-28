package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.LimitSnapshot;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelControlAction;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelControlReason;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelRuntimeStatus;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRuntimeStatusRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import com.huawei.devbridge.relaycontroller.interfaces.request.TunnelStatusItemRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.TunnelStatusReportRequest;
import com.huawei.devbridge.relaycontroller.interfaces.response.DataPlaneLimitsResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelStatusDecisionResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelStatusReportResponse;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.function.Function;
import java.util.stream.Collectors;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class TunnelStatusAppService {
    private final LocalClusterService localClusterService;
    private final TunnelRepository tunnelRepository;
    private final TunnelRuntimeStatusRepository statusRepository;
    private final BillingService billingService;
    private final RelayProperties relayProperties;

    public TunnelStatusReportResponse report(String clusterId, TunnelStatusReportRequest request) {
        localClusterService.requireLocalCluster(clusterId);
        assertNoDuplicateTunnels(request.getTunnels());
        long now = TimeUtils.nowSeconds();
        validateReportTime(request.getReportedAt(), now);
        if (request.getTunnels().isEmpty()) {
            return response(List.of());
        }
        List<String> tunnelIds = request.getTunnels().stream()
                .map(TunnelStatusItemRequest::getTunnelId)
                .toList();
        Map<String, Tunnel> tunnels = tunnelRepository
                .findByTunnelIdsAndRegion(tunnelIds, relayProperties.getRegion()).stream()
                .collect(Collectors.toMap(Tunnel::getTunnelId, Function.identity()));

        Map<Long, LimitSnapshot> snapshots = new HashMap<>();
        List<TunnelRuntimeStatus> statuses = new ArrayList<>();
        List<TunnelStatusDecisionResponse> decisions = new ArrayList<>();
        for (TunnelStatusItemRequest item : request.getTunnels()) {
            Tunnel tunnel = tunnels.get(item.getTunnelId());
            if (tunnel == null) {
                decisions.add(disconnect(item.getTunnelId(), TunnelControlReason.TUNNEL_NOT_FOUND));
                continue;
            }
            if (!clusterId.equals(tunnel.getClusterId())) {
                decisions.add(disconnect(item.getTunnelId(), TunnelControlReason.CLUSTER_MISMATCH));
                continue;
            }
            validateSession(item);
            statuses.add(toRuntimeStatus(clusterId, request, item));
            TunnelStatusDecisionResponse decision = decide(tunnel, item, snapshots, now);
            decisions.add(decision);
            if (decision.getAction() == TunnelControlAction.KEEP && isActive(item)) {
                tunnelRepository.refreshExpirationFromHeartbeat(
                        tunnel.getTunnelId(),
                        relayProperties.getRegion(),
                        now,
                        relayProperties.getBilling().getActivityRefreshIntervalSeconds());
            }
        }
        statusRepository.upsertAll(statuses);
        return response(decisions);
    }

    private TunnelStatusReportResponse response(List<TunnelStatusDecisionResponse> decisions) {
        return TunnelStatusReportResponse.builder()
                .nextReportInSeconds(relayProperties.getBilling().getStatusReportIntervalSeconds())
                .tunnels(decisions)
                .build();
    }

    private TunnelStatusDecisionResponse decide(
            Tunnel tunnel,
            TunnelStatusItemRequest item,
            Map<Long, LimitSnapshot> snapshots,
            long now) {
        if (tunnel.getExpiration() == null || tunnel.getExpiration() <= now) {
            return disconnect(tunnel.getTunnelId(), TunnelControlReason.TUNNEL_EXPIRED);
        }
        if (tunnel.getAccountId() == null) {
            return disconnect(tunnel.getTunnelId(), TunnelControlReason.ACCOUNT_UNAVAILABLE);
        }
        LimitSnapshot snapshot = snapshots.computeIfAbsent(
                tunnel.getAccountId(), billingService::currentSnapshot);
        BillingPlan plan = snapshot.plan();
        DataPlaneLimitsResponse limits = DataPlaneLimitsResponse.from(plan);
        if (!snapshot.account().isActive()) {
            return disconnect(tunnel.getTunnelId(), TunnelControlReason.ACCOUNT_DISABLED, snapshot, limits);
        }
        if (relayProperties.getBilling().isEnforcementEnabled() && snapshot.exhausted()) {
            return disconnect(tunnel.getTunnelId(), TunnelControlReason.QUOTA_EXCEEDED, snapshot, limits);
        }
        if (plan.getMaxHostsPerTunnel() > 0
                && item.getHostConnections() > plan.getMaxHostsPerTunnel()) {
            return disconnect(tunnel.getTunnelId(), TunnelControlReason.HOST_LIMIT_EXCEEDED, snapshot, limits);
        }
        return TunnelStatusDecisionResponse.builder()
                .tunnelId(tunnel.getTunnelId())
                .action(TunnelControlAction.KEEP)
                .reason(TunnelControlReason.NONE)
                .remainingBytes(snapshot.remainingBytes())
                .limits(limits)
                .build();
    }

    private static TunnelRuntimeStatus toRuntimeStatus(
            String clusterId,
            TunnelStatusReportRequest request,
            TunnelStatusItemRequest item) {
        return TunnelRuntimeStatus.builder()
                .tunnelId(item.getTunnelId())
                .clusterId(clusterId)
                .gatewayId(request.getGatewayId())
                .sessionId(item.getSessionId())
                .hostConnections(item.getHostConnections())
                .clientConnections(item.getClientConnections())
                .channelCount(item.getChannelCount())
                .uploadBytesPerSecond(item.getUploadBytesPerSecond())
                .downloadBytesPerSecond(item.getDownloadBytesPerSecond())
                .reportedAt(request.getReportedAt())
                .build();
    }

    private static void assertNoDuplicateTunnels(List<TunnelStatusItemRequest> items) {
        Set<String> tunnelIds = new HashSet<>();
        if (items.stream().anyMatch(item -> !tunnelIds.add(item.getTunnelId()))) {
            throw new BizException(ErrorCode.PARAM_INVALID, "duplicate tunnelId in status report");
        }
    }

    private static void validateSession(TunnelStatusItemRequest item) {
        if (item.getHostConnections() > 0
                && (item.getSessionId() == null || item.getSessionId().isBlank())) {
            throw new BizException(ErrorCode.PARAM_INVALID,
                    "sessionId is required when a host is connected");
        }
    }

    private void validateReportTime(long reportedAt, long now) {
        long maxSkew = Math.max(0, relayProperties.getBilling().getStatusMaxClockSkewSeconds());
        if (reportedAt < now - maxSkew || reportedAt > now + maxSkew) {
            throw new BizException(ErrorCode.PARAM_INVALID, "reportedAt exceeds allowed clock skew");
        }
    }

    private static boolean isActive(TunnelStatusItemRequest item) {
        return item.getHostConnections() > 0
                || item.getClientConnections() > 0
                || item.getChannelCount() > 0;
    }

    private static TunnelStatusDecisionResponse disconnect(
            String tunnelId, TunnelControlReason reason) {
        return TunnelStatusDecisionResponse.builder()
                .tunnelId(tunnelId)
                .action(TunnelControlAction.DISCONNECT)
                .reason(reason)
                .build();
    }

    private static TunnelStatusDecisionResponse disconnect(
            String tunnelId,
            TunnelControlReason reason,
            LimitSnapshot snapshot,
            DataPlaneLimitsResponse limits) {
        return TunnelStatusDecisionResponse.builder()
                .tunnelId(tunnelId)
                .action(TunnelControlAction.DISCONNECT)
                .reason(reason)
                .remainingBytes(snapshot.remainingBytes())
                .limits(limits)
                .build();
    }
}
