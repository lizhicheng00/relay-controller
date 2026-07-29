package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.application.service.BillingSettlementService.SettlementResult;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
@Slf4j
public class BillingSettlementJob {
    private final BillingSettlementService settlementService;
    private final RelayProperties relayProperties;

    @Scheduled(cron = "${relay.billing.settlement-cron:0 * * * * *}", zone = "UTC")
    public void settleUsage() {
        if (!relayProperties.getBilling().isSettlementEnabled()) {
            return;
        }
        int settled = 0;
        int batchSize = Math.max(1, relayProperties.getBilling().getSettlementBatchSize());
        for (int attempt = 0; attempt < batchSize; attempt++) {
            SettlementResult result = settlementService.settleNext();
            if (result != SettlementResult.SETTLED) {
                break;
            }
            settled++;
        }
        if (settled > 0) {
            log.info("Billing settlement completed: windows={}", settled);
        }
    }
}
