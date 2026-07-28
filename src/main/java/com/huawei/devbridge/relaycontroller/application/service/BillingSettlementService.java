package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.UsageWindow;
import com.huawei.devbridge.relaycontroller.domain.repository.BillingRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import java.util.List;
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
    public SettlementResult settleNext() {
        List<UsageWindow> windows = billingRepository.findUnbilledUsage(relayProperties.getRegion(), 1);
        if (windows.isEmpty()) {
            return SettlementResult.EMPTY;
        }
        UsageWindow window = windows.get(0);
        long usageBytes = window.getUsageBytes();
        long billedBytes = window.getBilledBytes();
        if (usageBytes <= billedBytes) {
            return SettlementResult.CONTENDED;
        }
        long now = TimeUtils.nowSeconds();
        if (!billingRepository.advanceBilledBytes(window.getId(), billedBytes, usageBytes, now)) {
            return SettlementResult.CONTENDED;
        }

        long delta = usageBytes - billedBytes;
        BillingPeriod period = billingService.ensurePeriod(window.getAccountId(), window.getWindowStart());
        if (!billingRepository.increasePeriodUsage(
                window.getAccountId(), period.getPeriodStart(), delta, now)) {
            throw new BizException(ErrorCode.INTERNAL_ERROR, "billing period update failed");
        }
        billingRepository.increaseTenMinuteUsage(
                window.getAccountId(), window.getTunnelId(), window.getWindowStart(), delta, now);
        tunnelRepository.increaseBandwidthUsed(window.getTunnelId(), relayProperties.getRegion(), delta, now);
        return SettlementResult.SETTLED;
    }

    public enum SettlementResult {
        EMPTY,
        CONTENDED,
        SETTLED
    }
}
