package com.huawei.devbridge.relaycontroller.infrastructure.persistence.mapper;

import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.UsageWindow;
import java.util.List;
import org.apache.ibatis.annotations.Param;

public interface BillingMapper {
    int insertAccountIfAbsent(
            @Param("namespace") String namespace,
            @Param("planCode") String planCode,
            @Param("now") long now);

    BillingAccount selectAccountByNamespace(@Param("namespace") String namespace);

    BillingAccount selectAccountById(@Param("accountId") Long accountId);

    BillingAccount selectAccountByNamespaceForUpdate(@Param("namespace") String namespace);

    BillingAccount selectAccountByIdForUpdate(@Param("accountId") Long accountId);

    BillingPlan selectPlanByCode(@Param("planCode") String planCode);

    int insertPeriodIfAbsent(
            @Param("accountId") Long accountId,
            @Param("periodStart") long periodStart,
            @Param("periodEnd") long periodEnd,
            @Param("quotaBytes") long quotaBytes,
            @Param("now") long now);

    BillingPeriod selectPeriod(
            @Param("accountId") Long accountId,
            @Param("periodStart") long periodStart);

    long sumPendingUsage(
            @Param("accountId") Long accountId,
            @Param("periodStart") long periodStart,
            @Param("periodEnd") long periodEnd);

    List<UsageWindow> selectUnbilledUsage(
            @Param("region") String region,
            @Param("limit") int limit);

    int advanceBilledBytes(
            @Param("usageWindowId") Long usageWindowId,
            @Param("expectedBilledBytes") long expectedBilledBytes,
            @Param("expectedUsageBytes") long expectedUsageBytes,
            @Param("now") long now);

    int increasePeriodUsage(
            @Param("accountId") Long accountId,
            @Param("periodStart") long periodStart,
            @Param("usageBytes") long usageBytes,
            @Param("now") long now);

    int increaseTenMinuteUsage(
            @Param("accountId") Long accountId,
            @Param("tunnelId") String tunnelId,
            @Param("windowStart") long windowStart,
            @Param("usageBytes") long usageBytes,
            @Param("now") long now);
}
