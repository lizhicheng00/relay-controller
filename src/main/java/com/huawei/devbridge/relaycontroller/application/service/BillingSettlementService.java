package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.MeteringRecord;
import com.huawei.devbridge.relaycontroller.domain.repository.BillingRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
@RequiredArgsConstructor
public class BillingSettlementService {
    private final BillingRepository billingRepository;
    private final BillingService billingService;
    private final TunnelRepository tunnelRepository;
    private final LocalClusterService localClusterService;
    private final RelayProperties relayProperties;

    @Transactional
    public int settleBatch(int limit) {
        List<String> clusterIds = localClusterService.localClusterIds();
        if (clusterIds.isEmpty()) {
            return 0;
        }
        List<MeteringRecord> records =
                billingRepository.lockUnsettledMetering(clusterIds, Math.max(1, limit));
        if (records.isEmpty()) {
            return 0;
        }

        Map<MinuteKey, Long> usageByMinute = new TreeMap<>(Comparator
                .comparing(MinuteKey::accountId)
                .thenComparing(MinuteKey::tunnelId)
                .thenComparing(MinuteKey::windowStart));
        Map<PeriodKey, Long> usageByPeriod = new TreeMap<>(Comparator
                .comparing(PeriodKey::accountId)
                .thenComparing(PeriodKey::periodStart));
        Map<String, Long> usageByTunnel = new TreeMap<>();
        for (MeteringRecord record : records) {
            long usageBytes = record.getUsageBytes();
            usageByMinute.merge(
                    new MinuteKey(
                            record.getAccountId(),
                            record.getTunnelId(),
                            minuteStart(record.getReportedAt())),
                    usageBytes,
                    Math::addExact);
            usageByPeriod.merge(
                    new PeriodKey(
                            record.getAccountId(),
                            BillingService.periodStart(record.getReportedAt())),
                    usageBytes,
                    Math::addExact);
            usageByTunnel.merge(record.getTunnelId(), usageBytes, Math::addExact);
        }

        for (Map.Entry<PeriodKey, Long> entry : usageByPeriod.entrySet()) {
            PeriodKey period = entry.getKey();
            billingService.ensurePeriod(period.accountId(), period.periodStart());
            if (!billingRepository.increasePeriodUsage(
                    period.accountId(), period.periodStart(), entry.getValue())) {
                throw new BizException(ErrorCode.INTERNAL_ERROR, "billing period update failed");
            }
        }
        for (Map.Entry<MinuteKey, Long> entry : usageByMinute.entrySet()) {
            MinuteKey key = entry.getKey();
            billingRepository.increaseMinuteUsage(
                    key.accountId(), key.tunnelId(), key.windowStart(), entry.getValue());
        }

        long settledAt = TimeUtils.nowSeconds();
        for (Map.Entry<String, Long> entry : usageByTunnel.entrySet()) {
            tunnelRepository.increaseBandwidthUsed(
                    entry.getKey(), relayProperties.getRegion(), entry.getValue(), settledAt);
        }
        for (PeriodKey period : usageByPeriod.keySet()) {
            billingRepository.blockQuotaIfExhausted(period.accountId(), period.periodStart());
        }
        if (billingRepository.markMeteringSettled(records) != records.size()) {
            throw new BizException(ErrorCode.INTERNAL_ERROR, "metering settlement marker update failed");
        }
        return records.size();
    }

    private static long minuteStart(long timestamp) {
        return timestamp - timestamp % 60;
    }

    private record MinuteKey(Long accountId, String tunnelId, long windowStart) {
    }

    private record PeriodKey(Long accountId, long periodStart) {
    }
}
