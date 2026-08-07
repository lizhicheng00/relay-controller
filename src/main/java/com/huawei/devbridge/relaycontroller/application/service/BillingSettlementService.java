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

        Map<PeriodKey, Long> usageByPeriod = new TreeMap<>(Comparator
                .comparing(PeriodKey::accountId)
                .thenComparing(PeriodKey::periodStart));
        Map<String, TunnelUsage> usageByTunnel = new TreeMap<>();
        for (MeteringRecord record : records) {
            long usageBytes = record.totalBytes();
            if (usageBytes <= 0) {
                continue;
            }
            usageByPeriod.merge(
                    new PeriodKey(
                            record.getAccountId(),
                            BillingService.periodStart(record.getReportedAt())),
                    usageBytes,
                    Math::addExact);
            usageByTunnel.merge(
                    record.getTunnelId(),
                    new TunnelUsage(usageBytes, record.getReportedAt()),
                    TunnelUsage::merge);
        }

        for (Map.Entry<PeriodKey, Long> entry : usageByPeriod.entrySet()) {
            PeriodKey period = entry.getKey();
            billingService.ensurePeriod(period.accountId(), period.periodStart());
            if (!billingRepository.increasePeriodUsage(
                    period.accountId(), period.periodStart(), entry.getValue())) {
                throw new BizException(ErrorCode.INTERNAL_ERROR, "billing period update failed");
            }
        }
        long settledAt = TimeUtils.nowSeconds();
        for (Map.Entry<String, TunnelUsage> entry : usageByTunnel.entrySet()) {
            TunnelUsage usage = entry.getValue();
            tunnelRepository.increaseBandwidthUsed(
                    entry.getKey(), relayProperties.getRegion(), usage.bytes(), settledAt);
            tunnelRepository.refreshExpiration(
                    entry.getKey(), relayProperties.getRegion(), usage.lastReportedAt());
        }
        for (PeriodKey period : usageByPeriod.keySet()) {
            billingRepository.blockQuotaIfExhausted(period.accountId(), period.periodStart());
        }
        if (billingRepository.markMeteringSettled(records) != records.size()) {
            throw new BizException(ErrorCode.INTERNAL_ERROR, "metering settlement marker update failed");
        }
        return records.size();
    }

    private record PeriodKey(Long accountId, long periodStart) {
    }

    private record TunnelUsage(long bytes, long lastReportedAt) {
        private TunnelUsage merge(TunnelUsage other) {
            return new TunnelUsage(
                    Math.addExact(bytes, other.bytes),
                    Math.max(lastReportedAt, other.lastReportedAt));
        }
    }
}
