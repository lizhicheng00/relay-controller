package com.huawei.devbridge.relaycontroller.infrastructure.persistence.repository;

import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.UsageWindow;
import com.huawei.devbridge.relaycontroller.domain.repository.BillingRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.mapper.BillingMapper;
import java.util.List;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;

@Repository
@RequiredArgsConstructor
public class BillingRepositoryImpl implements BillingRepository {
    private final BillingMapper billingMapper;

    @Override
    public void createAccountIfAbsent(String namespace, String planCode, long now) {
        billingMapper.insertAccountIfAbsent(namespace, planCode, now);
    }

    @Override
    public BillingAccount findAccountByNamespace(String namespace) {
        return billingMapper.selectAccountByNamespace(namespace);
    }

    @Override
    public BillingAccount findAccountById(Long accountId) {
        return billingMapper.selectAccountById(accountId);
    }

    @Override
    public BillingAccount lockAccountByNamespace(String namespace) {
        return billingMapper.selectAccountByNamespaceForUpdate(namespace);
    }

    @Override
    public BillingAccount lockAccountById(Long accountId) {
        return billingMapper.selectAccountByIdForUpdate(accountId);
    }

    @Override
    public BillingPlan findPlanByCode(String planCode) {
        return billingMapper.selectPlanByCode(planCode);
    }

    @Override
    public void createPeriodIfAbsent(
            Long accountId, long periodStart, long periodEnd, long quotaBytes, long now) {
        billingMapper.insertPeriodIfAbsent(accountId, periodStart, periodEnd, quotaBytes, now);
    }

    @Override
    public BillingPeriod findPeriod(Long accountId, long periodStart) {
        return billingMapper.selectPeriod(accountId, periodStart);
    }

    @Override
    public long sumPendingUsage(Long accountId, long periodStart, long periodEnd) {
        return billingMapper.sumPendingUsage(accountId, periodStart, periodEnd);
    }

    @Override
    public List<UsageWindow> findUnbilledUsage(String region, int limit) {
        return billingMapper.selectUnbilledUsage(region, limit);
    }

    @Override
    public boolean advanceBilledBytes(
            Long usageWindowId, long expectedBilledBytes, long expectedUsageBytes, long now) {
        return billingMapper.advanceBilledBytes(
                usageWindowId, expectedBilledBytes, expectedUsageBytes, now) == 1;
    }

    @Override
    public boolean increasePeriodUsage(Long accountId, long periodStart, long usageBytes, long now) {
        return billingMapper.increasePeriodUsage(accountId, periodStart, usageBytes, now) == 1;
    }

    @Override
    public void increaseTenMinuteUsage(
            Long accountId, String tunnelId, long windowStart, long usageBytes, long now) {
        billingMapper.increaseTenMinuteUsage(accountId, tunnelId, windowStart, usageBytes, now);
    }
}
