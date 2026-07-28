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
import com.huawei.devbridge.relaycontroller.application.service.LocalClusterService;
import com.huawei.devbridge.relaycontroller.application.service.TunnelStatusAppService;
import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPeriod;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.LimitSnapshot;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelControlAction;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelControlReason;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRuntimeStatusRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import com.huawei.devbridge.relaycontroller.interfaces.request.TunnelStatusItemRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.TunnelStatusReportRequest;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelStatusReportResponse;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
class TunnelStatusAppServiceTest {
    @Mock
    private LocalClusterService localClusterService;
    @Mock
    private TunnelRepository tunnelRepository;
    @Mock
    private TunnelRuntimeStatusRepository statusRepository;
    @Mock
    private BillingService billingService;

    @Test
    void activeTunnelReturnsLimitsAndRefreshesInactivityDeadline() {
        RelayProperties properties = new RelayProperties();
        TunnelStatusAppService service = new TunnelStatusAppService(
                localClusterService, tunnelRepository, statusRepository, billingService, properties);
        Tunnel tunnel = Tunnel.builder()
                .tunnelId("aaaadysa")
                .clusterId("cluster-a")
                .accountId(7L)
                .expiration(Math.toIntExact(TimeUtils.nowSeconds() + 3600))
                .build();
        when(tunnelRepository.findByTunnelIdsAndRegion(List.of("aaaadysa"), "region-a"))
                .thenReturn(List.of(tunnel));
        when(billingService.currentSnapshot(7L)).thenReturn(snapshot(false));

        TunnelStatusReportResponse response = service.report("cluster-a", request(1));

        assertThat(response.getNextReportInSeconds()).isEqualTo(10);
        assertThat(response.getTunnels()).singleElement().satisfies(decision -> {
            assertThat(decision.getAction()).isEqualTo(TunnelControlAction.KEEP);
            assertThat(decision.getReason()).isEqualTo(TunnelControlReason.NONE);
            assertThat(decision.getRemainingBytes()).isEqualTo(900L);
            assertThat(decision.getLimits().getMaxHostsPerTunnel()).isEqualTo(1);
        });
        verify(statusRepository).upsertAll(anyList());
        verify(tunnelRepository).refreshExpirationFromHeartbeat(
                eq("aaaadysa"), eq("region-a"), anyLong(), eq(300));
    }

    @Test
    void exhaustedAccountReturnsDisconnectDecision() {
        RelayProperties properties = new RelayProperties();
        TunnelStatusAppService service = new TunnelStatusAppService(
                localClusterService, tunnelRepository, statusRepository, billingService, properties);
        Tunnel tunnel = Tunnel.builder()
                .tunnelId("aaaadysa")
                .clusterId("cluster-a")
                .accountId(7L)
                .expiration(Math.toIntExact(TimeUtils.nowSeconds() + 3600))
                .build();
        when(tunnelRepository.findByTunnelIdsAndRegion(List.of("aaaadysa"), "region-a"))
                .thenReturn(List.of(tunnel));
        when(billingService.currentSnapshot(7L)).thenReturn(snapshot(true));

        TunnelStatusReportResponse response = service.report("cluster-a", request(0));

        assertThat(response.getTunnels()).singleElement().satisfies(decision -> {
            assertThat(decision.getAction()).isEqualTo(TunnelControlAction.DISCONNECT);
            assertThat(decision.getReason()).isEqualTo(TunnelControlReason.QUOTA_EXCEEDED);
        });
    }

    @Test
    void rejectsTimestampThatCouldPoisonLatestStatus() {
        RelayProperties properties = new RelayProperties();
        TunnelStatusAppService service = new TunnelStatusAppService(
                localClusterService, tunnelRepository, statusRepository, billingService, properties);
        TunnelStatusReportRequest request = request(0);
        request.setReportedAt(TimeUtils.nowSeconds() + 301);

        assertThatThrownBy(() -> service.report("cluster-a", request))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.PARAM_INVALID);
    }

    @Test
    void acceptsEmptyGatewayReportWithoutQueryingTunnels() {
        RelayProperties properties = new RelayProperties();
        TunnelStatusAppService service = new TunnelStatusAppService(
                localClusterService, tunnelRepository, statusRepository, billingService, properties);
        TunnelStatusReportRequest request = new TunnelStatusReportRequest();
        request.setGatewayId("gateway-a");
        request.setReportedAt(TimeUtils.nowSeconds());
        request.setTunnels(List.of());

        TunnelStatusReportResponse response = service.report("cluster-a", request);

        assertThat(response.getNextReportInSeconds()).isEqualTo(10);
        assertThat(response.getTunnels()).isEmpty();
        verify(tunnelRepository, never()).findByTunnelIdsAndRegion(anyList(), eq("region-a"));
        verify(statusRepository, never()).upsertAll(anyList());
    }

    private static TunnelStatusReportRequest request(int hostConnections) {
        TunnelStatusItemRequest item = new TunnelStatusItemRequest();
        item.setTunnelId("aaaadysa");
        item.setSessionId(hostConnections > 0 ? "session-a" : null);
        item.setHostConnections(hostConnections);
        item.setClientConnections(2);
        item.setChannelCount(3);
        item.setUploadBytesPerSecond(1024L);
        item.setDownloadBytesPerSecond(2048L);
        TunnelStatusReportRequest request = new TunnelStatusReportRequest();
        request.setGatewayId("gateway-a");
        request.setReportedAt(TimeUtils.nowSeconds());
        request.setTunnels(List.of(item));
        return request;
    }

    private static LimitSnapshot snapshot(boolean exhausted) {
        BillingAccount account = BillingAccount.builder()
                .id(7L)
                .namespace("ns-user-001")
                .planCode("trial")
                .status("active")
                .build();
        BillingPlan plan = BillingPlan.builder()
                .planCode("trial")
                .maxHostsPerTunnel(1)
                .maxTunnelBandwidthBytesPerSecond(5242880L)
                .maxHttpRequestsPerMinutePerPort(500)
                .maxConnectionsPerPort(100)
                .build();
        BillingPeriod period = BillingPeriod.builder().quotaBytes(1000L).billedBytes(100L).build();
        return new LimitSnapshot(
                account, plan, period, exhausted ? 1000L : 100L, exhausted ? 0L : 900L, exhausted);
    }
}
