package com.huawei.devbridge.relaycontroller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.huawei.devbridge.relaycontroller.application.service.BillingService;
import com.huawei.devbridge.relaycontroller.application.service.BillingSettlementService;
import com.huawei.devbridge.relaycontroller.application.service.LocalClusterService;
import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.domain.model.MeteringRecord;
import com.huawei.devbridge.relaycontroller.domain.repository.BillingRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import java.time.Instant;
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
    @Mock
    private LocalClusterService localClusterService;

    @Test
    void aggregatesRecordsByTunnelAndMinute() {
        BillingSettlementService service = service();
        List<MeteringRecord> records = List.of(
                record(11L, 400L, 1785206410L),
                record(12L, 600L, 1785206440L));
        when(billingRepository.lockUnsettledMetering(List.of("cluster-a"), 500)).thenReturn(records);
        when(billingRepository.increasePeriodUsage(7L, 1782864000L, 1000L)).thenReturn(true);
        when(billingRepository.markMeteringSettled(records)).thenReturn(2);

        assertThat(service.settleBatch(500)).isEqualTo(2);

        verify(billingRepository).blockQuotaIfExhausted(7L, 1782864000L);
        verify(billingRepository).increaseMinuteUsage(7L, "aaaadysa", 1785206400L, 1000L);
        verify(tunnelRepository).increaseBandwidthUsed(
                eq("aaaadysa"), eq("region-a"), eq(1000L), anyLong());
        verify(tunnelRepository).refreshExpiration("aaaadysa", "region-a", 1785206440L);
        verify(billingRepository).markMeteringSettled(records);
    }

    @Test
    void returnsWithoutWritesWhenNoMeteringIsPending() {
        BillingSettlementService service = service();
        when(billingRepository.lockUnsettledMetering(List.of("cluster-a"), 1)).thenReturn(List.of());

        assertThat(service.settleBatch(0)).isZero();

        verify(billingRepository, never()).increasePeriodUsage(anyLong(), anyLong(), anyLong());
        verify(billingRepository, never()).markMeteringSettled(anyList());
    }

    @Test
    void skipsSettlementWhenRegionHasNoClusters() {
        BillingSettlementService service = new BillingSettlementService(
                billingRepository, billingService, tunnelRepository, localClusterService, new RelayProperties());
        when(localClusterService.localClusterIds()).thenReturn(List.of());

        assertThat(service.settleBatch(500)).isZero();

        verify(billingRepository, never()).lockUnsettledMetering(anyList(), anyInt());
    }

    @Test
    void updatesQuotaStateOncePerAccountPeriod() {
        BillingSettlementService service = service();
        List<MeteringRecord> records = List.of(
                record(11L, "aaaadysa", 400L, 1785206410L),
                record(12L, "aaaadyta", 600L, 1785206470L));
        when(billingRepository.lockUnsettledMetering(List.of("cluster-a"), 500)).thenReturn(records);
        when(billingRepository.increasePeriodUsage(eq(7L), eq(1782864000L), anyLong()))
                .thenReturn(true);
        when(billingRepository.markMeteringSettled(records)).thenReturn(2);

        assertThat(service.settleBatch(500)).isEqualTo(2);

        verify(billingService).ensurePeriod(7L, 1782864000L);
        verify(billingRepository).increasePeriodUsage(7L, 1782864000L, 1000L);
        verify(billingRepository, times(1)).blockQuotaIfExhausted(7L, 1782864000L);
        verify(tunnelRepository).refreshExpiration("aaaadysa", "region-a", 1785206410L);
        verify(tunnelRepository).refreshExpiration("aaaadyta", "region-a", 1785206470L);
    }

    @Test
    void aggregatesTunnelAndPeriodUsageAcrossMinutes() {
        BillingSettlementService service = service();
        List<MeteringRecord> records = List.of(
                record(11L, 400L, 1785206410L),
                record(12L, 600L, 1785206470L));
        when(billingRepository.lockUnsettledMetering(List.of("cluster-a"), 500)).thenReturn(records);
        when(billingRepository.increasePeriodUsage(7L, 1782864000L, 1000L)).thenReturn(true);
        when(billingRepository.markMeteringSettled(records)).thenReturn(2);

        assertThat(service.settleBatch(500)).isEqualTo(2);

        verify(billingRepository).increaseMinuteUsage(7L, "aaaadysa", 1785206400L, 400L);
        verify(billingRepository).increaseMinuteUsage(7L, "aaaadysa", 1785206460L, 600L);
        verify(tunnelRepository).increaseBandwidthUsed(
                eq("aaaadysa"), eq("region-a"), eq(1000L), anyLong());
        verify(tunnelRepository).refreshExpiration("aaaadysa", "region-a", 1785206470L);
    }

    @Test
    void separatesUsageAcrossUtcBillingPeriods() {
        long julyStart = Instant.parse("2026-07-01T00:00:00Z").getEpochSecond();
        long augustStart = Instant.parse("2026-08-01T00:00:00Z").getEpochSecond();
        BillingSettlementService service = service();
        List<MeteringRecord> records = List.of(
                record(11L, 400L, Instant.parse("2026-07-31T23:59:50Z").getEpochSecond()),
                record(12L, 600L, Instant.parse("2026-08-01T00:00:10Z").getEpochSecond()));
        when(billingRepository.lockUnsettledMetering(List.of("cluster-a"), 500)).thenReturn(records);
        when(billingRepository.increasePeriodUsage(7L, julyStart, 400L)).thenReturn(true);
        when(billingRepository.increasePeriodUsage(7L, augustStart, 600L)).thenReturn(true);
        when(billingRepository.markMeteringSettled(records)).thenReturn(2);

        assertThat(service.settleBatch(500)).isEqualTo(2);

        verify(billingRepository).blockQuotaIfExhausted(7L, julyStart);
        verify(billingRepository).blockQuotaIfExhausted(7L, augustStart);
        verify(tunnelRepository).refreshExpiration(
                "aaaadysa", "region-a", Instant.parse("2026-08-01T00:00:10Z").getEpochSecond());
    }

    @Test
    void settlesZeroUsageWithoutBillingOrRefreshingExpiration() {
        BillingSettlementService service = service();
        List<MeteringRecord> records = List.of(record(11L, 0L, 1785206410L));
        when(billingRepository.lockUnsettledMetering(List.of("cluster-a"), 500)).thenReturn(records);
        when(billingRepository.markMeteringSettled(records)).thenReturn(1);

        assertThat(service.settleBatch(500)).isEqualTo(1);

        verify(billingService, never()).ensurePeriod(anyLong(), anyLong());
        verify(billingRepository, never()).increasePeriodUsage(anyLong(), anyLong(), anyLong());
        verify(billingRepository, never()).increaseMinuteUsage(anyLong(), eq("aaaadysa"), anyLong(), anyLong());
        verify(billingRepository, never()).blockQuotaIfExhausted(anyLong(), anyLong());
        verify(tunnelRepository, never()).increaseBandwidthUsed(eq("aaaadysa"), eq("region-a"), anyLong(), anyLong());
        verify(tunnelRepository, never()).refreshExpiration(eq("aaaadysa"), eq("region-a"), anyLong());
        verify(billingRepository).markMeteringSettled(records);
    }

    @Test
    void failsWhenNotAllLockedRecordsAreMarkedSettled() {
        BillingSettlementService service = service();
        List<MeteringRecord> records = List.of(
                record(11L, 400L, 1785206410L),
                record(12L, 600L, 1785206440L));
        when(billingRepository.lockUnsettledMetering(List.of("cluster-a"), 500)).thenReturn(records);
        when(billingRepository.increasePeriodUsage(7L, 1782864000L, 1000L)).thenReturn(true);
        when(billingRepository.markMeteringSettled(records)).thenReturn(1);

        assertThatThrownBy(() -> service.settleBatch(500))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.INTERNAL_ERROR);
    }

    @Test
    void failsWhenBillingPeriodCannotBeUpdated() {
        BillingSettlementService service = service();
        when(billingRepository.lockUnsettledMetering(List.of("cluster-a"), 500))
                .thenReturn(List.of(record(11L, 1000L, 1785206410L)));

        assertThatThrownBy(() -> service.settleBatch(500))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.INTERNAL_ERROR);
        verify(billingRepository, never()).increaseMinuteUsage(anyLong(), eq("aaaadysa"), anyLong(), anyLong());
        verify(billingRepository, never()).blockQuotaIfExhausted(anyLong(), anyLong());
        verify(billingRepository, never()).markMeteringSettled(anyList());
    }

    private BillingSettlementService service() {
        when(localClusterService.localClusterIds()).thenReturn(List.of("cluster-a"));
        return new BillingSettlementService(
                billingRepository, billingService, tunnelRepository, localClusterService, new RelayProperties());
    }

    private static MeteringRecord record(long id, long usageBytes, long reportedAt) {
        return record(id, "aaaadysa", usageBytes, reportedAt);
    }

    private static MeteringRecord record(long id, String tunnelId, long usageBytes, long reportedAt) {
        long uploadBytes = usageBytes / 2;
        return MeteringRecord.builder()
                .id(id)
                .accountId(7L)
                .tunnelId(tunnelId)
                .uploadBytes(uploadBytes)
                .downloadBytes(usageBytes - uploadBytes)
                .reportedAt(reportedAt)
                .build();
    }
}
