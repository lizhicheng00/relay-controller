package com.huawei.devbridge.relaycontroller.domain.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class BillingPeriod {
    private Long id;
    private Long accountId;
    private Long periodStart;
    private Long periodEnd;
    private Long quotaBytes;
    private Long billedBytes;
    private Long createdAt;
    private Long updatedAt;
}
