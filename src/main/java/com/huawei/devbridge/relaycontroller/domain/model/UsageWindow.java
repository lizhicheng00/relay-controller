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
    private String tunnelId;
    private Long windowStart;
    private Long usageBytes;
    private Long billedBytes;
}
