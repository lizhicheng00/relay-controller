package com.huawei.devbridge.relaycontroller.infrastructure.persistence.repository;

import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.MeteringRecord;
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
    public void createAccountIfAbsent(String namespace, String planCode) {
        billingMapper.insertAccountIfAbsent(namespace, planCode);
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
    public void createPeriodIfAbsent(Long accountId, long periodStart, long periodEnd, long quotaBytes) {
        billingMapper.insertPeriodIfAbsent(accountId, periodStart, periodEnd, quotaBytes);
    }

    @Override
    public BillingPeriod findPeriod(Long accountId, long periodStart) {
        return billingMapper.selectPeriod(accountId, periodStart);
    }

    @Override
    public List<MeteringRecord> lockUnsettledMetering(List<String> clusterIds, int limit) {
        return billingMapper.selectUnsettledMeteringForUpdate(clusterIds, limit);
    }

    @Override
    public boolean increasePeriodUsage(Long accountId, long periodStart, long usageBytes) {
        return billingMapper.increasePeriodUsage(accountId, periodStart, usageBytes) == 1;
    }

    @Override
    public void blockQuotaIfExhausted(Long accountId, long periodStart) {
        billingMapper.blockQuotaIfExhausted(accountId, periodStart);
    }

    @Override
    public int markMeteringSettled(List<MeteringRecord> records) {
        return billingMapper.markMeteringSettled(records);
    }
}
