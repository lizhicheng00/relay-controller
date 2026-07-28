package com.huawei.devbridge.relaycontroller.domain.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class UsageWindow {
    private Long id;
    private Long accountId;
    private String clusterId;
    private String tunnelId;
    private String sessionId;
    private Long windowStart;
    private Long usageBytes;
    private Long billedBytes;
    private Boolean sessionEnded;
    private Long reportedAt;
    private Long createdAt;
    private Long updatedAt;
}
