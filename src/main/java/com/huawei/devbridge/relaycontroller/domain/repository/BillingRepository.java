package com.huawei.devbridge.relaycontroller.domain.repository;

import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.UsageWindow;

public interface BillingRepository {
    void createAccountIfAbsent(String namespace, String planCode);

    BillingAccount findAccountById(Long accountId);

    BillingAccount lockAccountByNamespace(String namespace);

    BillingAccount lockAccountById(Long accountId);

    BillingPlan findPlanByCode(String planCode);

    void createPeriodIfAbsent(Long accountId, long periodStart, long periodEnd, long quotaBytes);

    BillingPeriod findPeriod(Long accountId, long periodStart);

    long sumPendingUsage(Long accountId, long periodStart, long periodEnd);

    UsageWindow findNextUnbilledUsage(String region);

    boolean advanceBilledBytes(Long usageWindowId, long expectedBilledBytes, long expectedUsageBytes);

    boolean increasePeriodUsage(Long accountId, long periodStart, long usageBytes);

    void increaseMinuteUsage(Long accountId, String tunnelId, long windowStart, long usageBytes);
}
