package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.AccountPlan;
import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.LimitSnapshot;
import com.huawei.devbridge.relaycontroller.domain.repository.BillingRepository;
import com.huawei.devbridge.relaycontroller.domain.service.NamespaceService;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import java.time.Instant;
import java.time.YearMonth;
import java.time.ZoneOffset;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
@RequiredArgsConstructor
public class BillingService {
    private final BillingRepository billingRepository;
    private final NamespaceService namespaceService;
    private final RelayProperties relayProperties;

    @Transactional
    public AccountPlan lockAccountForQuota(String rawNamespace) {
        String namespace = namespaceService.requireNamespace(rawNamespace);
        ensureAccount(namespace);
        BillingAccount account = billingRepository.lockAccountByNamespace(namespace);
        return new AccountPlan(requireActive(account), requirePlan(account.getPlanCode()));
    }

    @Transactional
    public AccountPlan accountPlan(Long accountId) {
        BillingAccount account = billingRepository.findAccountById(accountId);
        return new AccountPlan(requireActive(account), requirePlan(account.getPlanCode()));
    }

    @Transactional
    public LimitSnapshot currentSnapshot(String rawNamespace) {
        String namespace = namespaceService.requireNamespace(rawNamespace);
        ensureAccount(namespace);
        return snapshot(
                requireAccount(billingRepository.lockAccountByNamespace(namespace)),
                TimeUtils.nowSeconds());
    }

    @Transactional
    public BillingPeriod ensurePeriod(Long accountId, long timestamp) {
        BillingAccount account = requireAccount(billingRepository.lockAccountById(accountId));
        BillingPlan plan = requirePlan(account.getPlanCode());
        PeriodRange range = periodRange(timestamp);
        billingRepository.createPeriodIfAbsent(
                accountId, range.start(), range.end(), plan.getMonthlyQuotaBytes());
        BillingPeriod period = billingRepository.findPeriod(accountId, range.start());
        if (period == null) {
            throw new BizException(ErrorCode.INTERNAL_ERROR, "billing period unavailable");
        }
        return period;
    }

    @Transactional
    public void assertTrafficAllowed(String namespace) {
        LimitSnapshot snapshot = currentSnapshot(namespace);
        if (!snapshot.account().isActive()) {
            throw new BizException(ErrorCode.ACCOUNT_DISABLED);
        }
        if (relayProperties.getBilling().isEnforcementEnabled() && snapshot.exhausted()) {
            throw new BizException(ErrorCode.ACCOUNT_QUOTA_EXCEEDED);
        }
    }

    private void ensureAccount(String namespace) {
        billingRepository.createAccountIfAbsent(
                namespace, relayProperties.getBilling().getDefaultPlanCode());
    }

    private LimitSnapshot snapshot(BillingAccount account, long now) {
        BillingPlan plan = requirePlan(account.getPlanCode());
        PeriodRange range = periodRange(now);
        billingRepository.createPeriodIfAbsent(
                account.getId(), range.start(), range.end(), plan.getMonthlyQuotaBytes());
        BillingPeriod period = billingRepository.findPeriod(account.getId(), range.start());
        if (period == null) {
            throw new BizException(ErrorCode.INTERNAL_ERROR, "billing period unavailable");
        }
        long pending = billingRepository.sumPendingUsage(account.getId(), range.start(), range.end());
        long used = saturatedAdd(period.getBilledBytes(), pending);
        long remaining = Math.max(0, period.getQuotaBytes() - Math.min(period.getQuotaBytes(), used));
        return new LimitSnapshot(
                account, plan, period, used, remaining, used >= period.getQuotaBytes());
    }

    private static BillingAccount requireAccount(BillingAccount account) {
        if (account == null) {
            throw new BizException(ErrorCode.INTERNAL_ERROR, "billing account unavailable");
        }
        return account;
    }

    private static BillingAccount requireActive(BillingAccount account) {
        BillingAccount required = requireAccount(account);
        if (!required.isActive()) {
            throw new BizException(ErrorCode.ACCOUNT_DISABLED);
        }
        return required;
    }

    private BillingPlan requirePlan(String planCode) {
        BillingPlan plan = billingRepository.findPlanByCode(planCode);
        if (plan == null) {
            throw new BizException(ErrorCode.INTERNAL_ERROR, "billing plan unavailable");
        }
        return plan;
    }

    private static PeriodRange periodRange(long timestamp) {
        YearMonth month = YearMonth.from(Instant.ofEpochSecond(timestamp).atZone(ZoneOffset.UTC));
        long start = month.atDay(1).atStartOfDay(ZoneOffset.UTC).toEpochSecond();
        long end = month.plusMonths(1).atDay(1).atStartOfDay(ZoneOffset.UTC).toEpochSecond();
        return new PeriodRange(start, end);
    }

    private static long saturatedAdd(long left, long right) {
        if (right > Long.MAX_VALUE - left) {
            return Long.MAX_VALUE;
        }
        return left + right;
    }

    private record PeriodRange(long start, long end) {
    }
}
