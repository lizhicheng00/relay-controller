package com.huawei.devbridge.relaycontroller.domain.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class BillingAccount {
    private Long id;
    private String namespace;
    private String planCode;
    private String status;

    public boolean isActive() {
        return "active".equalsIgnoreCase(status);
    }
}
