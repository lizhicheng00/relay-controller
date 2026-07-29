package com.huawei.devbridge.relaycontroller.interfaces.response;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class LimitsResponse {
    private Long resetAt;
    private Long quotaBytes;
    private Long remainingBytes;
    private Long activeTunnels;
    private Integer maxTunnels;
    private Integer maxPortsPerTunnel;
    private Integer maxHostsPerTunnel;
    private Long maxTunnelBandwidthBytesPerSecond;
    private Integer maxHttpRequestsPerMinutePerPort;
    private Integer maxConnectionsPerPort;
}
