package com.huawei.devbridge.relaycontroller.interfaces.response;

import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class DataPlaneLimitsResponse {
    private Integer maxHostsPerTunnel;
    private Long maxTunnelBandwidthBytesPerSecond;
    private Integer maxHttpRequestsPerMinutePerPort;
    private Integer maxConnectionsPerPort;

    public static DataPlaneLimitsResponse from(BillingPlan plan) {
        return builder()
                .maxHostsPerTunnel(plan.getMaxHostsPerTunnel())
                .maxTunnelBandwidthBytesPerSecond(plan.getMaxTunnelBandwidthBytesPerSecond())
                .maxHttpRequestsPerMinutePerPort(plan.getMaxHttpRequestsPerMinutePerPort())
                .maxConnectionsPerPort(plan.getMaxConnectionsPerPort())
                .build();
    }
}
