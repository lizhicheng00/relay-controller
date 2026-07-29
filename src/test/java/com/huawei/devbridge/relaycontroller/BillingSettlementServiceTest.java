package com.huawei.devbridge.relaycontroller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.huawei.devbridge.relaycontroller.application.service.BillingService;
import com.huawei.devbridge.relaycontroller.application.service.BillingSettlementService;
import com.huawei.devbridge.relaycontroller.application.service.BillingSettlementService.SettlementResult;
import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.UsageWindow;
import com.huawei.devbridge.relaycontroller.domain.repository.BillingRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
class BillingSettlementServiceTest {
    @Mock
    private BillingRepository billingRepository;
    @Mock
    private BillingService billingService;
    @Mock
    private TunnelRepository tunnelRepository;

    @Test
    void settlesOnlyTheUnbilledDelta() {
        RelayProperties properties = new RelayProperties();
        BillingSettlementService service =
                new BillingSettlementService(billingRepository, billingService, tunnelRepository, properties);
        UsageWindow window = UsageWindow.builder()
                .id(11L)
                .accountId(7L)
                .tunnelId("aaaadysa")
                .windowStart(1785206400L)
                .usageBytes(1000L)
                .billedBytes(400L)
                .build();
        when(billingRepository.findNextUnbilledUsage("region-a")).thenReturn(window);
        when(billingRepository.advanceBilledBytes(eq(11L), eq(400L), eq(1000L)))
                .thenReturn(true);
        when(billingService.ensurePeriod(7L, 1785206400L)).thenReturn(BillingPeriod.builder()
                .periodStart(1782864000L)
                .build());
        when(billingRepository.increasePeriodUsage(eq(7L), eq(1782864000L), eq(600L)))
                .thenReturn(true);

        SettlementResult result = service.settleNext();

        assertThat(result).isEqualTo(SettlementResult.SETTLED);
        verify(billingRepository).increasePeriodUsage(eq(7L), eq(1782864000L), eq(600L));
        verify(billingRepository).increaseMinuteUsage(
                eq(7L),
                eq("aaaadysa"),
                eq(1785206400L),
                eq(600L));
        verify(tunnelRepository).increaseBandwidthUsed(
                eq("aaaadysa"),
                eq("region-a"),
                eq(600L),
                anyLong());
    }

    @Test
    void concurrentClaimPreventsDuplicateSettlement() {
        RelayProperties properties = new RelayProperties();
        BillingSettlementService service =
                new BillingSettlementService(billingRepository, billingService, tunnelRepository, properties);
        UsageWindow window = UsageWindow.builder()
                .id(11L)
                .accountId(7L)
                .tunnelId("aaaadysa")
                .windowStart(1785206400L)
                .usageBytes(1000L)
                .billedBytes(400L)
                .build();
        when(billingRepository.findNextUnbilledUsage("region-a")).thenReturn(window);
        when(billingRepository.advanceBilledBytes(eq(11L), eq(400L), eq(1000L)))
                .thenReturn(false);

        assertThat(service.settleNext()).isEqualTo(SettlementResult.CONTENDED);
        verify(billingRepository, never()).increasePeriodUsage(anyLong(), anyLong(), anyLong());
        verify(tunnelRepository, never()).increaseBandwidthUsed(anyString(), anyString(), anyLong(), anyLong());
    }

    @Test
    void stopsSettlementWhenBillingPeriodCannotBeUpdated() {
        RelayProperties properties = new RelayProperties();
        BillingSettlementService service =
                new BillingSettlementService(billingRepository, billingService, tunnelRepository, properties);
        UsageWindow window = UsageWindow.builder()
                .id(11L)
                .accountId(7L)
                .tunnelId("aaaadysa")
                .windowStart(1785206400L)
                .usageBytes(1000L)
                .billedBytes(400L)
                .build();
        when(billingRepository.findNextUnbilledUsage("region-a")).thenReturn(window);
        when(billingRepository.advanceBilledBytes(eq(11L), eq(400L), eq(1000L)))
                .thenReturn(true);
        when(billingService.ensurePeriod(7L, 1785206400L)).thenReturn(BillingPeriod.builder()
                .periodStart(1782864000L)
                .build());

        assertThatThrownBy(service::settleNext)
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.INTERNAL_ERROR);
        verify(billingRepository, never())
                .increaseMinuteUsage(anyLong(), anyString(), anyLong(), anyLong());
        verify(tunnelRepository, never()).increaseBandwidthUsed(anyString(), anyString(), anyLong(), anyLong());
    }
}
