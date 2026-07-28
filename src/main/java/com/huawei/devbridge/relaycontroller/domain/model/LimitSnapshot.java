package com.huawei.devbridge.relaycontroller.domain.model;

public record LimitSnapshot(
        BillingAccount account,
        BillingPlan plan,
        BillingPeriod period,
        long usedBytes,
        long remainingBytes,
        boolean exhausted) {
}
