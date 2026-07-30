package com.huawei.devbridge.relaycontroller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.huawei.devbridge.relaycontroller.application.service.BillingService;
import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.LimitSnapshot;
import com.huawei.devbridge.relaycontroller.domain.repository.BillingRepository;
import com.huawei.devbridge.relaycontroller.domain.service.NamespaceService;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import java.time.Instant;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
class BillingServiceTest {
    @Mock
    private BillingRepository billingRepository;

    @Test
    void currentSnapshotUsesSettledPeriodUsage() {
        BillingService service = service();
        stubAccountAndPlan(1000L);
        when(billingRepository.findPeriod(eq(7L), anyLong())).thenReturn(BillingPeriod.builder()
                .periodStart(1782864000L)
                .periodEnd(1785542400L)
                .quotaBytes(1000L)
                .billedBytes(400L)
                .build());
        LimitSnapshot snapshot = service.currentSnapshot("ns-user-001");

        assertThat(snapshot.usedBytes()).isEqualTo(400L);
        assertThat(snapshot.remainingBytes()).isEqualTo(600L);
        assertThat(snapshot.exhausted()).isFalse();
    }

    @Test
    void tokenIssuanceIsRejectedWhenSettledUsageReachesQuota() {
        BillingService service = service();
        stubAccountAndPlan(1000L);
        when(billingRepository.findPeriod(eq(7L), anyLong())).thenReturn(BillingPeriod.builder()
                .periodStart(1782864000L)
                .periodEnd(1785542400L)
                .quotaBytes(1000L)
                .billedBytes(1000L)
                .build());

        assertThatThrownBy(() -> service.assertTrafficAllowed("ns-user-001"))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.ACCOUNT_QUOTA_EXCEEDED);
    }

    @Test
    void disabledAccountIsRejectedEvenWhenQuotaEnforcementIsDisabled() {
        RelayProperties properties = new RelayProperties();
        properties.getBilling().setEnforcementEnabled(false);
        BillingService service = new BillingService(
                billingRepository, new NamespaceService(), properties);
        BillingAccount account = BillingAccount.builder()
                .id(7L)
                .namespace("ns-user-001")
                .planCode("trial")
                .status("disabled")
                .build();
        when(billingRepository.lockAccountByNamespace("ns-user-001")).thenReturn(account);
        when(billingRepository.findPlanByCode("trial")).thenReturn(BillingPlan.builder()
                .planCode("trial")
                .monthlyQuotaBytes(1000L)
                .build());
        when(billingRepository.findPeriod(eq(7L), anyLong())).thenReturn(BillingPeriod.builder()
                .quotaBytes(1000L)
                .billedBytes(0L)
                .build());

        assertThatThrownBy(() -> service.assertTrafficAllowed("ns-user-001"))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.ACCOUNT_DISABLED);
    }

    @Test
    void createsPeriodUsingUtcMonthBoundaries() {
        BillingService service = service();
        long julyStart = Instant.parse("2026-07-01T00:00:00Z").getEpochSecond();
        long augustStart = Instant.parse("2026-08-01T00:00:00Z").getEpochSecond();
        BillingAccount account = BillingAccount.builder()
                .id(7L)
                .planCode("trial")
                .status("active")
                .build();
        BillingPlan plan = BillingPlan.builder()
                .planCode("trial")
                .monthlyQuotaBytes(1000L)
                .build();
        BillingPeriod period = BillingPeriod.builder()
                .periodStart(julyStart)
                .periodEnd(augustStart)
                .quotaBytes(1000L)
                .billedBytes(0L)
                .build();
        when(billingRepository.lockAccountById(7L)).thenReturn(account);
        when(billingRepository.findPlanByCode("trial")).thenReturn(plan);
        when(billingRepository.findPeriod(7L, julyStart)).thenReturn(period);

        service.ensurePeriod(7L, Instant.parse("2026-07-31T23:59:59Z").getEpochSecond());

        verify(billingRepository).createPeriodIfAbsent(7L, julyStart, augustStart, 1000L);
    }

    private BillingService service() {
        return new BillingService(billingRepository, new NamespaceService(), new RelayProperties());
    }

    private void stubAccountAndPlan(long quotaBytes) {
        BillingAccount account = BillingAccount.builder()
                .id(7L)
                .namespace("ns-user-001")
                .planCode("trial")
                .status("active")
                .build();
        when(billingRepository.lockAccountByNamespace("ns-user-001")).thenReturn(account);
        when(billingRepository.findPlanByCode("trial")).thenReturn(BillingPlan.builder()
                .planCode("trial")
                .monthlyQuotaBytes(quotaBytes)
                .build());
    }
}
