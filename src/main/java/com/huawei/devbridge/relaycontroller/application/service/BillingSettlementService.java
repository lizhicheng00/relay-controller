package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
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
    private final RelayProperties relayProperties;

    @Transactional
    public int settleBatch(int limit) {
        List<MeteringRecord> records =
                billingRepository.lockUnsettledMetering(relayProperties.getRegion(), Math.max(1, limit));
        if (records.isEmpty()) {
            return 0;
        }

        Map<SettlementKey, Long> usageByMinute = new TreeMap<>(Comparator
                .comparing(SettlementKey::accountId)
                .thenComparing(SettlementKey::tunnelId)
                .thenComparing(SettlementKey::windowStart));
        for (MeteringRecord record : records) {
            SettlementKey key = new SettlementKey(
                    record.getAccountId(),
                    record.getTunnelId(),
                    minuteStart(record.getReportedAt()));
            usageByMinute.merge(key, record.getUsageBytes(), Math::addExact);
        }

        long settledAt = TimeUtils.nowSeconds();
        for (Map.Entry<SettlementKey, Long> entry : usageByMinute.entrySet()) {
            settle(entry.getKey(), entry.getValue(), settledAt);
        }
        List<Long> recordIds = records.stream().map(MeteringRecord::getId).toList();
        if (billingRepository.markMeteringSettled(recordIds) != recordIds.size()) {
            throw new BizException(ErrorCode.INTERNAL_ERROR, "metering settlement marker update failed");
        }
        return records.size();
    }

    private void settle(SettlementKey key, long usageBytes, long settledAt) {
        BillingPeriod period = billingService.ensurePeriod(key.accountId(), key.windowStart());
        if (!billingRepository.increasePeriodUsage(key.accountId(), period.getPeriodStart(), usageBytes)) {
            throw new BizException(ErrorCode.INTERNAL_ERROR, "billing period update failed");
        }
        billingRepository.increaseMinuteUsage(
                key.accountId(), key.tunnelId(), key.windowStart(), usageBytes);
        tunnelRepository.increaseBandwidthUsed(
                key.tunnelId(), relayProperties.getRegion(), usageBytes, settledAt);
    }

    private static long minuteStart(long timestamp) {
        return timestamp - timestamp % 60;
    }

    private record SettlementKey(Long accountId, String tunnelId, long windowStart) {
    }
}
