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
import java.time.ZoneId;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
@RequiredArgsConstructor
public class BillingService {
    private static final ZoneId BILLING_ZONE = ZoneId.of("Asia/Shanghai");

    private final BillingRepository billingRepository;
    private final NamespaceService namespaceService;
    private final RelayProperties relayProperties;

    @Transactional
    public AccountPlan lockAccountForQuota(String rawAccountNamespace) {
        String accountNamespace = namespaceService.requireNamespace(rawAccountNamespace);
        ensureAccount(accountNamespace);
        BillingAccount account = billingRepository.lockAccountByNamespace(accountNamespace);
        return new AccountPlan(requireActive(account), requirePlan(account.getPlanCode()));
    }

    @Transactional
    public AccountPlan accountPlan(Long accountId) {
        BillingAccount account = billingRepository.findAccountById(accountId);
        return new AccountPlan(requireActive(account), requirePlan(account.getPlanCode()));
    }

    @Transactional
    public LimitSnapshot currentSnapshot(String rawAccountNamespace) {
        String accountNamespace = namespaceService.requireNamespace(rawAccountNamespace);
        ensureAccount(accountNamespace);
        return snapshot(
                requireAccount(billingRepository.lockAccountByNamespace(accountNamespace)),
                TimeUtils.nowSeconds());
    }

    @Transactional
    public void ensurePeriod(Long accountId, long timestamp) {
        BillingAccount account = requireAccount(billingRepository.lockAccountById(accountId));
        BillingPlan plan = requirePlan(account.getPlanCode());
        getOrCreatePeriod(account.getId(), plan, timestamp);
    }

    @Transactional
    public void assertTrafficAllowed(Long accountId) {
        BillingAccount account = requireAccount(billingRepository.lockAccountById(accountId));
        LimitSnapshot snapshot = snapshot(account, TimeUtils.nowSeconds());
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
        BillingPeriod period = getOrCreatePeriod(account.getId(), plan, now);
        long used = period.getBilledBytes();
        long remaining = Math.max(0, period.getQuotaBytes() - Math.min(period.getQuotaBytes(), used));
        return new LimitSnapshot(
                account, plan, period, used, remaining, used >= period.getQuotaBytes());
    }

    private BillingPeriod getOrCreatePeriod(Long accountId, BillingPlan plan, long timestamp) {
        PeriodRange range = periodRange(timestamp);
        billingRepository.createPeriodIfAbsent(
                accountId, range.start(), range.end(), plan.getMonthlyQuotaBytes());
        BillingPeriod period = billingRepository.findPeriod(accountId, range.start());
        if (period == null) {
            throw new BizException(ErrorCode.INTERNAL_ERROR, "billing period unavailable");
        }
        return period;
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

    static long periodStart(long timestamp) {
        return periodRange(timestamp).start();
    }

    private static PeriodRange periodRange(long timestamp) {
        YearMonth month = YearMonth.from(Instant.ofEpochSecond(timestamp).atZone(BILLING_ZONE));
        long start = month.atDay(1).atStartOfDay(BILLING_ZONE).toEpochSecond();
        long end = month.plusMonths(1).atDay(1).atStartOfDay(BILLING_ZONE).toEpochSecond();
        return new PeriodRange(start, end);
    }

    private record PeriodRange(long start, long end) {
    }
}
