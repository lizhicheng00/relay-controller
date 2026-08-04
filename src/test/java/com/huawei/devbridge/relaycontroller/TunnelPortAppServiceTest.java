package com.huawei.devbridge.relaycontroller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.huawei.devbridge.relaycontroller.application.service.BillingService;
import com.huawei.devbridge.relaycontroller.application.service.LocalClusterService;
import com.huawei.devbridge.relaycontroller.application.service.TunnelPortAppService;
import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.domain.model.AccountPlan;
import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelPort;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelProtocol;
import com.huawei.devbridge.relaycontroller.domain.repository.ClusterRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelPortRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.domain.service.NamespaceService;
import com.huawei.devbridge.relaycontroller.domain.service.TunnelDomainService;
import com.huawei.devbridge.relaycontroller.domain.service.TunnelPortDomainService;
import com.huawei.devbridge.relaycontroller.interfaces.request.CreateTunnelPortRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.UpdateTunnelPortRequest;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelPortResponse;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
class TunnelPortAppServiceTest {
    @Mock
    private TunnelRepository tunnelRepository;
    @Mock
    private TunnelPortRepository tunnelPortRepository;
    @Mock
    private ClusterRepository clusterRepository;
    @Mock
    private BillingService billingService;

    @Test
    void createTunnelPortWritesPolicy() {
        TunnelPortAppService service = newService();
        CreateTunnelPortRequest request = new CreateTunnelPortRequest();
        request.setPort(8080L);
        request.setProtocol(TunnelProtocol.HTTP);
        request.setAllowAnonymous(false);

        when(tunnelRepository.findByTunnelIdAndClustersForUpdate("aaaadysa", List.of("cluster-a")))
                .thenReturn(tunnel("ns-user-001", "cluster-a"));
        when(billingService.accountPlan(7L)).thenReturn(accountPlan());
        when(tunnelPortRepository.save(org.mockito.ArgumentMatchers.any(TunnelPort.class)))
                .thenAnswer(invocation -> invocation.getArgument(0));

        TunnelPortResponse response = service.create("ns-user-001", "aaaadysa", request);

        assertThat(response.getTunnelId()).isEqualTo("aaaadysa");
        assertThat(response.getTunnelCode()).isEqualTo(123456L);
        assertThat(response.getPort()).isEqualTo(8080L);
        assertThat(response.getProtocol()).isEqualTo(TunnelProtocol.HTTP);
        assertThat(response.getAllowAnonymous()).isFalse();
        verify(tunnelRepository).refreshExpiration(eq("aaaadysa"), eq("region-a"), anyLong());
    }

    @Test
    void createTunnelPortRejectsDuplicatePort() {
        TunnelPortAppService service = newService();
        CreateTunnelPortRequest request = new CreateTunnelPortRequest();
        request.setPort(8080L);
        request.setProtocol(TunnelProtocol.AUTO);
        request.setAllowAnonymous(false);

        when(tunnelRepository.findByTunnelIdAndClustersForUpdate("aaaadysa", List.of("cluster-a")))
                .thenReturn(tunnel("ns-user-001", "cluster-a"));
        when(tunnelPortRepository.existsByTunnelCodeAndPort(123456L, 8080L)).thenReturn(true);

        assertThatThrownBy(() -> service.create("ns-user-001", "aaaadysa", request))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.TUNNEL_PORT_ALREADY_EXISTS);
    }

    @Test
    void createTunnelPortRejectsEleventhPort() {
        TunnelPortAppService service = newService();
        CreateTunnelPortRequest request = new CreateTunnelPortRequest();
        request.setPort(8080L);
        request.setProtocol(TunnelProtocol.AUTO);
        request.setAllowAnonymous(false);

        when(tunnelRepository.findByTunnelIdAndClustersForUpdate("aaaadysa", List.of("cluster-a")))
                .thenReturn(tunnel("ns-user-001", "cluster-a"));
        when(billingService.accountPlan(7L)).thenReturn(accountPlan());
        when(tunnelPortRepository.countByTunnelCode(123456L)).thenReturn(10L);

        assertThatThrownBy(() -> service.create("ns-user-001", "aaaadysa", request))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.TUNNEL_PORT_QUOTA_EXCEEDED);
    }

    @Test
    void createTunnelPortRejectsInvalidPort() {
        TunnelPortAppService service = newService();
        CreateTunnelPortRequest request = new CreateTunnelPortRequest();
        request.setPort(65536L);
        request.setProtocol(TunnelProtocol.AUTO);
        request.setAllowAnonymous(false);

        when(tunnelRepository.findByTunnelIdAndClustersForUpdate("aaaadysa", List.of("cluster-a")))
                .thenReturn(tunnel("ns-user-001", "cluster-a"));

        assertThatThrownBy(() -> service.create("ns-user-001", "aaaadysa", request))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.TUNNEL_PORT_INVALID);
    }

    @Test
    void createTunnelPortRejectsNullPort() {
        TunnelPortAppService service = newService();
        CreateTunnelPortRequest request = new CreateTunnelPortRequest();
        request.setAllowAnonymous(false);

        when(tunnelRepository.findByTunnelIdAndClustersForUpdate("aaaadysa", List.of("cluster-a")))
                .thenReturn(tunnel("ns-user-001", "cluster-a"));

        assertThatThrownBy(() -> service.create("ns-user-001", "aaaadysa", request))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.TUNNEL_PORT_INVALID);
    }

    @Test
    void createTunnelPortRejectsNamespaceMismatch() {
        TunnelPortAppService service = newService();
        CreateTunnelPortRequest request = new CreateTunnelPortRequest();
        request.setPort(8080L);
        request.setProtocol(TunnelProtocol.AUTO);
        request.setAllowAnonymous(false);

        when(tunnelRepository.findByTunnelIdAndClustersForUpdate("aaaadysa", List.of("cluster-a")))
                .thenReturn(tunnel("ns-user-a", "cluster-a"));

        assertThatThrownBy(() -> service.create("ns-user-b", "aaaadysa", request))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.TUNNEL_ACCESS_DENIED);
    }

    @Test
    void listTunnelPortsReturnsPolicies() {
        TunnelPortAppService service = newService();

        when(tunnelRepository.findByTunnelIdAndRegion("aaaadysa", "region-a"))
                .thenReturn(tunnel("ns-user-001", "cluster-a"));
        when(tunnelPortRepository.findByTunnelCode(123456L)).thenReturn(List.of(
                TunnelPort.builder().tunnelCode(123456L).port(8080L).protocol(TunnelProtocol.HTTP).allowAnonymous(false).build(),
                TunnelPort.builder().tunnelCode(123456L).port(8888L).protocol(TunnelProtocol.HTTPS).allowAnonymous(true).build()));

        List<TunnelPortResponse> response = service.list("ns-user-001", "aaaadysa");

        assertThat(response).extracting(TunnelPortResponse::getPort).containsExactly(8080L, 8888L);
    }

    @Test
    void updateTunnelPortChangesPolicy() {
        TunnelPortAppService service = newService();
        UpdateTunnelPortRequest request = new UpdateTunnelPortRequest();
        request.setProtocol(TunnelProtocol.HTTPS);
        request.setAllowAnonymous(true);

        when(tunnelRepository.findByTunnelIdAndClustersForUpdate("aaaadysa", List.of("cluster-a")))
                .thenReturn(tunnel("ns-user-001", "cluster-a"));
        when(tunnelPortRepository.findByTunnelCodeAndPort(123456L, 8080L))
                .thenReturn(TunnelPort.builder().tunnelCode(123456L).port(8080L)
                        .protocol(TunnelProtocol.HTTP).allowAnonymous(false).build());

        TunnelPortResponse response = service.update("ns-user-001", "aaaadysa", 8080L, request);

        assertThat(response.getAllowAnonymous()).isTrue();
        assertThat(response.getProtocol()).isEqualTo(TunnelProtocol.HTTPS);
        verify(tunnelPortRepository).updatePolicy(123456L, 8080L, TunnelProtocol.HTTPS, true);
        verify(tunnelRepository).refreshExpiration(eq("aaaadysa"), eq("region-a"), anyLong());
    }

    @Test
    void updateTunnelPortKeepsProtocolWhenOmitted() {
        TunnelPortAppService service = newService();
        UpdateTunnelPortRequest request = new UpdateTunnelPortRequest();
        request.setAllowAnonymous(true);

        when(tunnelRepository.findByTunnelIdAndClustersForUpdate("aaaadysa", List.of("cluster-a")))
                .thenReturn(tunnel("ns-user-001", "cluster-a"));
        when(tunnelPortRepository.findByTunnelCodeAndPort(123456L, 8080L))
                .thenReturn(TunnelPort.builder().tunnelCode(123456L).port(8080L)
                        .protocol(TunnelProtocol.HTTP).allowAnonymous(false).build());

        TunnelPortResponse response = service.update("ns-user-001", "aaaadysa", 8080L, request);

        assertThat(response.getProtocol()).isEqualTo(TunnelProtocol.HTTP);
        assertThat(response.getAllowAnonymous()).isTrue();
        verify(tunnelPortRepository).updatePolicy(123456L, 8080L, TunnelProtocol.HTTP, true);
    }

    @Test
    void updateTunnelPortKeepsAllowAnonymousWhenOmitted() {
        TunnelPortAppService service = newService();
        UpdateTunnelPortRequest request = new UpdateTunnelPortRequest();
        request.setProtocol(TunnelProtocol.HTTPS);

        when(tunnelRepository.findByTunnelIdAndClustersForUpdate("aaaadysa", List.of("cluster-a")))
                .thenReturn(tunnel("ns-user-001", "cluster-a"));
        when(tunnelPortRepository.findByTunnelCodeAndPort(123456L, 8080L))
                .thenReturn(TunnelPort.builder().tunnelCode(123456L).port(8080L)
                        .protocol(TunnelProtocol.HTTP).allowAnonymous(false).build());

        TunnelPortResponse response = service.update("ns-user-001", "aaaadysa", 8080L, request);

        assertThat(response.getProtocol()).isEqualTo(TunnelProtocol.HTTPS);
        assertThat(response.getAllowAnonymous()).isFalse();
        verify(tunnelPortRepository).updatePolicy(123456L, 8080L, TunnelProtocol.HTTPS, false);
    }

    @Test
    void deleteTunnelPortRemovesPolicy() {
        TunnelPortAppService service = newService();

        when(tunnelRepository.findByTunnelIdAndClustersForUpdate("aaaadysa", List.of("cluster-a")))
                .thenReturn(tunnel("ns-user-001", "cluster-a"));
        when(tunnelPortRepository.findByTunnelCodeAndPort(123456L, 8080L))
                .thenReturn(TunnelPort.builder().tunnelCode(123456L).port(8080L).allowAnonymous(false).build());

        Boolean deleted = service.delete("ns-user-001", "aaaadysa", 8080L);

        assertThat(deleted).isTrue();
        verify(tunnelPortRepository).deleteByTunnelCodeAndPort(123456L, 8080L);
        verify(tunnelRepository).refreshExpiration(eq("aaaadysa"), eq("region-a"), anyLong());
    }

    @Test
    void detailRejectsMissingPort() {
        TunnelPortAppService service = newService();

        when(tunnelRepository.findByTunnelIdAndRegion("aaaadysa", "region-a")).thenReturn(tunnel("ns-user-001", "cluster-a"));
        assertThatThrownBy(() -> service.detail("ns-user-001", "aaaadysa", 8080L))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.TUNNEL_PORT_NOT_FOUND);
    }

    private TunnelPortAppService newService() {
        RelayProperties properties = new RelayProperties();
        org.mockito.Mockito.lenient()
                .when(clusterRepository.findIdsByRegion("region-a"))
                .thenReturn(List.of("cluster-a"));
        return new TunnelPortAppService(
                tunnelRepository,
                tunnelPortRepository,
                new LocalClusterService(clusterRepository, properties),
                new NamespaceService(),
                new TunnelDomainService(),
                new TunnelPortDomainService(),
                properties,
                billingService);
    }

    private Tunnel tunnel(String namespace, String clusterId) {
        return Tunnel.builder()
                .tunnelId("aaaadysa")
                .tunnelCode(123456L)
                .namespace(namespace)
                .accountId(7L)
                .clusterId(clusterId)
                .deleted(0)
                .build();
    }

    private static AccountPlan accountPlan() {
        return new AccountPlan(
                BillingAccount.builder().id(7L).namespace("ns-user-001").planCode("trial").status("active").build(),
                BillingPlan.builder().planCode("trial").maxPortsPerTunnel(10).build());
    }
}
