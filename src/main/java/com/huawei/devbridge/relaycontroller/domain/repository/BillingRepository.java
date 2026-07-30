package com.huawei.devbridge.relaycontroller.domain.repository;

import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.MeteringRecord;
import java.util.List;

public interface BillingRepository {
    void createAccountIfAbsent(String namespace, String planCode);

    BillingAccount findAccountById(Long accountId);

    BillingAccount lockAccountByNamespace(String namespace);

    BillingAccount lockAccountById(Long accountId);

    BillingPlan findPlanByCode(String planCode);

    void createPeriodIfAbsent(Long accountId, long periodStart, long periodEnd, long quotaBytes);

    BillingPeriod findPeriod(Long accountId, long periodStart);

    List<MeteringRecord> lockUnsettledMetering(String region, int limit);

    boolean increasePeriodUsage(Long accountId, long periodStart, long usageBytes);

    void blockQuotaIfExhausted(Long accountId, long periodStart);

    void increaseMinuteUsage(Long accountId, String tunnelId, long windowStart, long usageBytes);

    int markMeteringSettled(List<MeteringRecord> records);
}
