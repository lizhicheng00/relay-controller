package com.huawei.devbridge.relaycontroller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.huawei.devbridge.relaycontroller.application.service.BillingService;
import com.huawei.devbridge.relaycontroller.application.service.BillingSettlementService;
import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.MeteringRecord;
import com.huawei.devbridge.relaycontroller.domain.repository.BillingRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import java.util.List;
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
    void aggregatesRecordsByTunnelAndMinute() {
        BillingSettlementService service = service();
        List<MeteringRecord> records = List.of(
                record(11L, 400L, 1785206410L),
                record(12L, 600L, 1785206440L));
        when(billingRepository.lockUnsettledMetering("region-a", 500)).thenReturn(records);
        when(billingService.ensurePeriod(7L, 1785206400L)).thenReturn(BillingPeriod.builder()
                .periodStart(1782864000L)
                .build());
        when(billingRepository.increasePeriodUsage(7L, 1782864000L, 1000L)).thenReturn(true);
        when(billingRepository.markMeteringSettled(List.of(11L, 12L))).thenReturn(2);

        assertThat(service.settleBatch(500)).isEqualTo(2);

        verify(billingRepository).increaseMinuteUsage(7L, "aaaadysa", 1785206400L, 1000L);
        verify(tunnelRepository).increaseBandwidthUsed(
                eq("aaaadysa"), eq("region-a"), eq(1000L), anyLong());
        verify(billingRepository).markMeteringSettled(List.of(11L, 12L));
    }

    @Test
    void returnsWithoutWritesWhenNoMeteringIsPending() {
        BillingSettlementService service = service();
        when(billingRepository.lockUnsettledMetering("region-a", 1)).thenReturn(List.of());

        assertThat(service.settleBatch(0)).isZero();

        verify(billingRepository, never()).increasePeriodUsage(anyLong(), anyLong(), anyLong());
        verify(billingRepository, never()).markMeteringSettled(anyList());
    }

    @Test
    void failsWhenNotAllLockedRecordsAreMarkedSettled() {
        BillingSettlementService service = service();
        List<MeteringRecord> records = List.of(
                record(11L, 400L, 1785206410L),
                record(12L, 600L, 1785206440L));
        when(billingRepository.lockUnsettledMetering("region-a", 500)).thenReturn(records);
        when(billingService.ensurePeriod(7L, 1785206400L)).thenReturn(BillingPeriod.builder()
                .periodStart(1782864000L)
                .build());
        when(billingRepository.increasePeriodUsage(7L, 1782864000L, 1000L)).thenReturn(true);
        when(billingRepository.markMeteringSettled(List.of(11L, 12L))).thenReturn(1);

        assertThatThrownBy(() -> service.settleBatch(500))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.INTERNAL_ERROR);
    }

    @Test
    void failsWhenBillingPeriodCannotBeUpdated() {
        BillingSettlementService service = service();
        when(billingRepository.lockUnsettledMetering("region-a", 500))
                .thenReturn(List.of(record(11L, 1000L, 1785206410L)));
        when(billingService.ensurePeriod(7L, 1785206400L)).thenReturn(BillingPeriod.builder()
                .periodStart(1782864000L)
                .build());

        assertThatThrownBy(() -> service.settleBatch(500))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.INTERNAL_ERROR);
        verify(billingRepository, never()).increaseMinuteUsage(anyLong(), eq("aaaadysa"), anyLong(), anyLong());
        verify(billingRepository, never()).markMeteringSettled(anyList());
    }

    private BillingSettlementService service() {
        return new BillingSettlementService(
                billingRepository, billingService, tunnelRepository, new RelayProperties());
    }

    private static MeteringRecord record(long id, long usageBytes, long reportedAt) {
        return MeteringRecord.builder()
                .id(id)
                .accountId(7L)
                .tunnelId("aaaadysa")
                .usageBytes(usageBytes)
                .reportedAt(reportedAt)
                .build();
    }
}
