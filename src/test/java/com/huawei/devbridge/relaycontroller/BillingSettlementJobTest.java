package com.huawei.devbridge.relaycontroller;

import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.huawei.devbridge.relaycontroller.application.service.BillingSettlementJob;
import com.huawei.devbridge.relaycontroller.application.service.BillingSettlementService;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
class BillingSettlementJobTest {
    @Mock
    private BillingSettlementService settlementService;

    @Test
    void drainsFullBatchesUntilTheBacklogIsEmpty() {
        RelayProperties properties = new RelayProperties();
        properties.getBilling().setSettlementBatchSize(500);
        when(settlementService.settleBatch(500)).thenReturn(500, 500, 40);

        new BillingSettlementJob(settlementService, properties).settleUsage();

        verify(settlementService, times(3)).settleBatch(500);
    }

    @Test
    void skipsSettlementWhenDisabled() {
        RelayProperties properties = new RelayProperties();
        properties.getBilling().setSettlementEnabled(false);

        new BillingSettlementJob(settlementService, properties).settleUsage();

        verify(settlementService, never()).settleBatch(anyInt());
    }
}
