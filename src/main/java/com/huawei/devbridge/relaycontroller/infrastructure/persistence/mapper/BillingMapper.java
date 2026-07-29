package com.huawei.devbridge.relaycontroller.infrastructure.persistence.mapper;

import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.MeteringRecord;
import java.util.List;
import org.apache.ibatis.annotations.Param;

public interface BillingMapper {
    int insertAccountIfAbsent(
            @Param("namespace") String namespace,
            @Param("planCode") String planCode);

    BillingAccount selectAccountById(@Param("accountId") Long accountId);

    BillingAccount selectAccountByNamespaceForUpdate(@Param("namespace") String namespace);

    BillingAccount selectAccountByIdForUpdate(@Param("accountId") Long accountId);

    BillingPlan selectPlanByCode(@Param("planCode") String planCode);

    int insertPeriodIfAbsent(
            @Param("accountId") Long accountId,
            @Param("periodStart") long periodStart,
            @Param("periodEnd") long periodEnd,
            @Param("quotaBytes") long quotaBytes);

    BillingPeriod selectPeriod(
            @Param("accountId") Long accountId,
            @Param("periodStart") long periodStart);

    List<MeteringRecord> selectUnsettledMeteringForUpdate(
            @Param("region") String region,
            @Param("limit") int limit);

    int increasePeriodUsage(
            @Param("accountId") Long accountId,
            @Param("periodStart") long periodStart,
            @Param("usageBytes") long usageBytes);

    int increaseMinuteUsage(
            @Param("accountId") Long accountId,
            @Param("tunnelId") String tunnelId,
            @Param("windowStart") long windowStart,
            @Param("usageBytes") long usageBytes);

    int markMeteringSettled(@Param("meteringIds") List<Long> meteringIds);
}
