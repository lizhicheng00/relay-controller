package com.huawei.devbridge.relaycontroller.domain.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class Tunnel {
    private Long id;
    private String name;
    private String tunnelId;
    private Long tunnelCode;
    private String clusterId;
    private Integer expiration;
    private Integer expirationHours;
    private String namespace;
    private Long accountId;
    private String description;
    private Long bandwidthUsed;
    private String url;
    private TunnelType type;
    private Integer deleted;
    private Long createdAt;
    private Long updatedAt;
    private Long portCount;
}
