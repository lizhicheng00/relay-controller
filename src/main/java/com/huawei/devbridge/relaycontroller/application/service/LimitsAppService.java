package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.LimitSnapshot;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.interfaces.response.LimitsResponse;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class LimitsAppService {
    private final BillingService billingService;
    private final TunnelRepository tunnelRepository;

    public LimitsResponse getLimits(String namespace) {
        LimitSnapshot snapshot = billingService.currentSnapshot(namespace);
        BillingPeriod period = snapshot.period();
        BillingPlan plan = snapshot.plan();
        long activeTunnels = tunnelRepository.countActiveByAccountId(
                snapshot.account().getId(), TimeUtils.nowSeconds());
        return LimitsResponse.builder()
                .resetAt(period.getPeriodEnd())
                .quotaBytes(period.getQuotaBytes())
                .remainingBytes(snapshot.remainingBytes())
                .activeTunnels(activeTunnels)
                .maxTunnels(plan.getMaxTunnels())
                .maxPortsPerTunnel(plan.getMaxPortsPerTunnel())
                .maxHostsPerTunnel(plan.getMaxHostsPerTunnel())
                .maxTunnelBandwidthBytesPerSecond(plan.getMaxTunnelBandwidthBytesPerSecond())
                .maxHttpRequestsPerMinutePerPort(plan.getMaxHttpRequestsPerMinutePerPort())
                .maxConnectionsPerPort(plan.getMaxConnectionsPerPort())
                .build();
    }
}
