package com.huawei.devbridge.relaycontroller.domain.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class BillingPlan {
    private String planCode;
    private Long monthlyQuotaBytes;
    private Integer maxTunnels;
    private Integer maxPortsPerTunnel;
    private Integer maxHostsPerTunnel;
    private Long maxTunnelBandwidthBytesPerSecond;
    private Integer maxHttpRequestsPerMinutePerPort;
    private Integer maxConnectionsPerPort;
}
