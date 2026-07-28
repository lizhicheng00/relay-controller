package com.huawei.devbridge.relaycontroller.domain.repository;

import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.UsageWindow;
import java.util.List;

public interface BillingRepository {
    void createAccountIfAbsent(String namespace, String planCode, long now);

    BillingAccount findAccountByNamespace(String namespace);

    BillingAccount findAccountById(Long accountId);

    BillingAccount lockAccountByNamespace(String namespace);

    BillingAccount lockAccountById(Long accountId);

    BillingPlan findPlanByCode(String planCode);

    void createPeriodIfAbsent(
            Long accountId, long periodStart, long periodEnd, long quotaBytes, long now);

    BillingPeriod findPeriod(Long accountId, long periodStart);

    long sumPendingUsage(Long accountId, long periodStart, long periodEnd);

    List<UsageWindow> findUnbilledUsage(String region, int limit);

    boolean advanceBilledBytes(
            Long usageWindowId, long expectedBilledBytes, long expectedUsageBytes, long now);

    boolean increasePeriodUsage(Long accountId, long periodStart, long usageBytes, long now);

    void increaseTenMinuteUsage(
            Long accountId, String tunnelId, long windowStart, long usageBytes, long now);
}
