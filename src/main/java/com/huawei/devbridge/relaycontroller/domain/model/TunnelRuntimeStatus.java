package com.huawei.devbridge.relaycontroller.domain.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class TunnelRuntimeStatus {
    private String tunnelId;
    private String clusterId;
    private String gatewayId;
    private String sessionId;
    private Integer hostConnections;
    private Integer clientConnections;
    private Integer channelCount;
    private Long uploadBytesPerSecond;
    private Long downloadBytesPerSecond;
    private Long reportedAt;
    private Long updatedAt;
}
