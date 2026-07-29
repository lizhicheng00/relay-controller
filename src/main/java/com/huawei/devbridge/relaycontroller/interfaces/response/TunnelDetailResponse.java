package com.huawei.devbridge.relaycontroller.interfaces.response;

import com.fasterxml.jackson.annotation.JsonInclude;
import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class TunnelDetailResponse {
    private String name;
    private String tunnelId;
    private Long tunnelCode;
    private String clusterId;
    private String description;
    private Long bandwidthUsed;
    private Integer expirationHours;
    private Long tunnelExpiration;
    private Long created;
    private String url;
    private String type;
    @JsonInclude(JsonInclude.Include.NON_NULL)
    private TunnelStatusResponse status;
}
