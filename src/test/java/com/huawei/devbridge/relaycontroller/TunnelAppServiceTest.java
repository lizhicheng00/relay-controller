package com.huawei.devbridge.relaycontroller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.ArgumentMatchers.isNull;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.huawei.devbridge.relaycontroller.application.service.BillingService;
import com.huawei.devbridge.relaycontroller.application.service.LocalClusterService;
import com.huawei.devbridge.relaycontroller.application.service.TunnelAppService;
import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.AccountPlan;
import com.huawei.devbridge.relaycontroller.domain.model.BillingAccount;
import com.huawei.devbridge.relaycontroller.domain.model.BillingPlan;
import com.huawei.devbridge.relaycontroller.domain.model.Cluster;
import com.huawei.devbridge.relaycontroller.domain.model.JwtScope;
import com.huawei.devbridge.relaycontroller.domain.model.JwtToken;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelRuntimeStatus;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelType;
import com.huawei.devbridge.relaycontroller.domain.repository.ClusterRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelPortRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRuntimeStatusRepository;
import com.huawei.devbridge.relaycontroller.domain.service.JwtTokenService;
import com.huawei.devbridge.relaycontroller.domain.service.NamespaceService;
import com.huawei.devbridge.relaycontroller.domain.service.TunnelCodeGenerator;
import com.huawei.devbridge.relaycontroller.domain.service.TunnelDomainService;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import com.huawei.devbridge.relaycontroller.interfaces.request.CreateTunnelRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.UpdateTunnelRequest;
import com.huawei.devbridge.relaycontroller.interfaces.response.CreateTunnelResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelDetailResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelListItemResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelTokenResponse;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
class TunnelAppServiceTest {
    @Mock
    private TunnelRepository tunnelRepository;
    @Mock
    private ClusterRepository clusterRepository;
    @Mock
    private JwtTokenService jwtTokenService;
    @Mock
    private TunnelPortRepository tunnelPortRepository;
    @Mock
    private TunnelRuntimeStatusRepository tunnelRuntimeStatusRepository;
    @Mock
    private BillingService billingService;

    @Test
    void createTunnelAllocatesCodeAndReturnsMetadata() {
        RelayProperties properties = new RelayProperties();
        properties.setDomain("myhuaweicloud.com");
        TunnelAppService service = new TunnelAppService(
                tunnelRepository,
                new LocalClusterService(clusterRepository, properties),
                new NamespaceService(),
                new FixedTunnelCodeGenerator(),
                jwtTokenService,
                new TunnelDomainService(),
                tunnelPortRepository,
                tunnelRuntimeStatusRepository,
                properties,
                billingService);
        when(billingService.lockAccountForQuota("ns-user-001")).thenReturn(accountPlan());
        CreateTunnelRequest request = new CreateTunnelRequest();
        request.setName("dev");
        request.setClusterId("cluster-a");
        when(clusterRepository.findByClusterIdAndRegion("cluster-a", "region-a"))
                .thenReturn(Cluster.builder().clusterId("cluster-a").region("region-a").build());
        when(tunnelRepository.existsByTunnelCode(123456L)).thenReturn(false);
        when(tunnelRepository.existsByTunnelId("aaaadysa")).thenReturn(false);
        when(tunnelRepository.save(org.mockito.ArgumentMatchers.any(Tunnel.class))).thenAnswer(invocation -> invocation.getArgument(0));
        CreateTunnelResponse response = service.createTunnel("ns-user-001", request);

        assertThat(response.getTunnelId()).isEqualTo("aaaadysa");
        assertThat(response.getTunnelCode()).isEqualTo(123456L);
        assertThat(response.getUrl()).isEqualTo("aaaadysa.cluster-a.myhuaweicloud.com");
        assertThat(response.getType()).isEqualTo("bridge");
        assertThat(response.getExpirationHours()).isEqualTo(72);
        assertThat(response.getTunnelExpiration()).isNotNull();
    }

    @Test
    void createTunnelRejectsClusterOutsideLocalRegion() {
        TunnelAppService service = newService(new RelayProperties());
        CreateTunnelRequest request = new CreateTunnelRequest();
        request.setName("dev");
        request.setClusterId("cluster-b");

        assertThatThrownBy(() -> service.createTunnel("ns-user-001", request))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.CLUSTER_NOT_FOUND);
    }

    @Test
    void createTunnelUsesCustomExpirationHours() {
        TunnelAppService service = newService(new RelayProperties());
        CreateTunnelRequest request = new CreateTunnelRequest();
        request.setName("dev");
        request.setClusterId("cluster-a");
        request.setExpiration(2);
        when(clusterRepository.findByClusterIdAndRegion("cluster-a", "region-a"))
                .thenReturn(Cluster.builder().clusterId("cluster-a").region("region-a").build());
        when(tunnelRepository.existsByTunnelCode(123456L)).thenReturn(false);
        when(tunnelRepository.existsByTunnelId("aaaadysa")).thenReturn(false);
        when(tunnelRepository.save(org.mockito.ArgumentMatchers.any(Tunnel.class))).thenAnswer(invocation -> invocation.getArgument(0));
        CreateTunnelResponse response = service.createTunnel("ns-user-001", request);

        assertThat(response.getExpirationHours()).isEqualTo(2);
    }

    @Test
    void createTunnelRejectsNonPositiveExpirationHours() {
        TunnelAppService service = newService(new RelayProperties());
        CreateTunnelRequest request = new CreateTunnelRequest();
        request.setName("dev");
        request.setClusterId("cluster-a");
        request.setExpiration(0);

        when(clusterRepository.findByClusterIdAndRegion("cluster-a", "region-a"))
                .thenReturn(Cluster.builder().clusterId("cluster-a").region("region-a").build());

        assertThatThrownBy(() -> service.createTunnel("ns-user-001", request))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.PARAM_INVALID);
    }

    @Test
    void createTunnelRejectsExpirationLongerThan30Days() {
        TunnelAppService service = newService(new RelayProperties());
        CreateTunnelRequest request = new CreateTunnelRequest();
        request.setName("dev");
        request.setClusterId("cluster-a");
        request.setExpiration(721);

        when(clusterRepository.findByClusterIdAndRegion("cluster-a", "region-a"))
                .thenReturn(Cluster.builder().clusterId("cluster-a").region("region-a").build());

        assertThatThrownBy(() -> service.createTunnel("ns-user-001", request))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.PARAM_INVALID);
    }

    @Test
    void createTunnelRejectsWhenNamespaceQuotaExceeded() {
        TunnelAppService service = newService(new RelayProperties());
        CreateTunnelRequest request = new CreateTunnelRequest();
        request.setName("dev");
        request.setClusterId("cluster-a");

        when(clusterRepository.findByClusterIdAndRegion("cluster-a", "region-a"))
                .thenReturn(Cluster.builder().clusterId("cluster-a").region("region-a").build());
        when(billingService.lockAccountForQuota("ns-user-001")).thenReturn(accountPlan());
        when(tunnelRepository.countActiveByAccountId(eq(7L), anyLong()))
                .thenReturn(10L);

        assertThatThrownBy(() -> service.createTunnel("ns-user-001", request))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.TUNNEL_QUOTA_EXCEEDED);
    }

    @Test
    void createTunnelUsesAccountScopedDatabaseQuota() {
        RelayProperties properties = new RelayProperties();
        TunnelAppService service = new TunnelAppService(
                tunnelRepository,
                new LocalClusterService(clusterRepository, properties),
                new NamespaceService(),
                new FixedTunnelCodeGenerator(),
                jwtTokenService,
                new TunnelDomainService(),
                tunnelPortRepository,
                tunnelRuntimeStatusRepository,
                properties,
                billingService);
        CreateTunnelRequest request = new CreateTunnelRequest();
        request.setName("dev");
        request.setClusterId("cluster-a");

        when(billingService.lockAccountForQuota("ns-user-001")).thenReturn(accountPlan());
        when(clusterRepository.findByClusterIdAndRegion("cluster-a", "region-a"))
                .thenReturn(Cluster.builder().clusterId("cluster-a").region("region-a").build());
        when(tunnelRepository.countActiveByAccountId(eq(7L), anyLong())).thenReturn(9L);
        when(tunnelRepository.existsByTunnelCode(anyLong())).thenReturn(false);
        when(tunnelRepository.existsByTunnelId(anyString())).thenReturn(false);
        when(tunnelRepository.save(org.mockito.ArgumentMatchers.any(Tunnel.class)))
                .thenAnswer(invocation -> invocation.getArgument(0));

        service.createTunnel("ns-user-001", request);

        verify(billingService).lockAccountForQuota("ns-user-001");
        verify(tunnelRepository).countActiveByAccountId(eq(7L), anyLong());
    }

    @Test
    void updateTunnelChangesExpiration() {
        TunnelAppService service = newService(new RelayProperties());
        UpdateTunnelRequest request = new UpdateTunnelRequest();
        request.setExpiration(1);
        Tunnel tunnel = Tunnel.builder()
                .tunnelId("aaaadysa")
                .namespace("ns-user-001")
                .clusterId("cluster-a")
                .deleted(0)
                .expiration(Math.toIntExact(TimeUtils.nowSeconds() + 1800))
                .build();

        when(tunnelRepository.findByTunnelIdAndRegionForUpdate("aaaadysa", "region-a")).thenReturn(tunnel);

        long before = TimeUtils.nowSeconds();
        Boolean updated = service.updateTunnel("ns-user-001", "aaaadysa", request);
        long after = TimeUtils.nowSeconds();

        assertThat(updated).isTrue();
        assertThat(tunnel.getExpiration())
                .isBetween(Math.toIntExact(before + 3600L), Math.toIntExact(after + 3600L));
        assertThat(tunnel.getExpirationHours()).isEqualTo(1);
        verify(tunnelRepository).update(tunnel);
    }

    @Test
    void updateTunnelRefreshesInactivityExpiration() {
        TunnelAppService service = newService(new RelayProperties());
        UpdateTunnelRequest request = new UpdateTunnelRequest();
        request.setDescription("updated");
        Tunnel tunnel = Tunnel.builder()
                .tunnelId("aaaadysa")
                .namespace("ns-user-001")
                .clusterId("cluster-a")
                .deleted(0)
                .expirationHours(2)
                .expiration(Math.toIntExact(TimeUtils.nowSeconds() + 60))
                .build();

        when(tunnelRepository.findByTunnelIdAndRegionForUpdate("aaaadysa", "region-a")).thenReturn(tunnel);

        long before = TimeUtils.nowSeconds();
        service.updateTunnel("ns-user-001", "aaaadysa", request);
        long after = TimeUtils.nowSeconds();

        assertThat(tunnel.getExpiration())
                .isBetween(Math.toIntExact(before + 7200L), Math.toIntExact(after + 7200L));
    }

    @Test
    void listTunnelsQueriesLocalRegionOnly() {
        TunnelAppService service = newService(new RelayProperties());
        Tunnel local = Tunnel.builder()
                .name("local")
                .namespace("ns-user-001")
                .clusterId("cluster-a")
                .url("local-cluster-a-myhuaweicloud.com")
                .portCount(2L)
                .deleted(0)
                .build();

        when(tunnelRepository.findActiveByNamespaceAndRegion(
                eq("ns-user-001"), isNull(), eq("region-a"), anyLong()))
                .thenReturn(List.of(local));

        List<TunnelListItemResponse> response = service.listTunnels("ns-user-001", null);

        assertThat(response).extracting(TunnelListItemResponse::getName).containsExactly("local");
        assertThat(response.get(0).getPortCount()).isEqualTo(2L);
    }

    @Test
    void listTunnelsQueriesActiveTunnelsOnly() {
        TunnelAppService service = newService(new RelayProperties());

        when(tunnelRepository.findActiveByNamespaceAndRegion(
                eq("ns-user-001"), isNull(), eq("region-a"), anyLong()))
                .thenReturn(List.of());

        List<TunnelListItemResponse> response = service.listTunnels("ns-user-001", null);

        assertThat(response).isEmpty();
    }

    @Test
    void tunnelDetailIncludesLatestRuntimeStatus() {
        TunnelAppService service = newService(new RelayProperties());
        Tunnel tunnel = Tunnel.builder()
                .tunnelId("aaaadysa")
                .namespace("ns-user-001")
                .expiration(Math.toIntExact(TimeUtils.nowSeconds() + 3600))
                .build();
        TunnelRuntimeStatus status = TunnelRuntimeStatus.builder()
                .hostConnections(1)
                .clientConnections(2)
                .channelCount(3)
                .uploadBytesPerSecond(1024L)
                .downloadBytesPerSecond(2048L)
                .reportedAt(1720000000L)
                .build();
        when(tunnelRepository.findByTunnelIdAndRegion("aaaadysa", "region-a")).thenReturn(tunnel);
        when(tunnelRuntimeStatusRepository.findByTunnelId("aaaadysa")).thenReturn(status);

        TunnelDetailResponse response = service.getTunnelDetail("ns-user-001", "aaaadysa");

        assertThat(response.getHostConnections()).isEqualTo(1);
        assertThat(response.getClientConnections()).isEqualTo(2);
        assertThat(response.getChannelCount()).isEqualTo(3);
        assertThat(response.getUploadBytesPerSecond()).isEqualTo(1024L);
        assertThat(response.getDownloadBytesPerSecond()).isEqualTo(2048L);
        assertThat(response.getStatusReportedAt()).isEqualTo(1720000000L);
    }

    @Test
    void updateTunnelStoresEnumType() {
        TunnelAppService service = newService(new RelayProperties());
        UpdateTunnelRequest request = new UpdateTunnelRequest();
        request.setType(TunnelType.ENV);
        Tunnel tunnel = Tunnel.builder()
                .tunnelId("aaaadysa")
                .namespace("ns-user-001")
                .clusterId("cluster-a")
                .deleted(0)
                .type(TunnelType.BRIDGE)
                .build();

        when(tunnelRepository.findByTunnelIdAndRegionForUpdate("aaaadysa", "region-a")).thenReturn(tunnel);

        Boolean updated = service.updateTunnel("ns-user-001", "aaaadysa", request);

        assertThat(updated).isTrue();
        assertThat(tunnel.getType()).isEqualTo(TunnelType.ENV);
        verify(tunnelRepository).update(tunnel);
    }

    @Test
    void deleteTunnelCleansTunnelPorts() {
        TunnelAppService service = newService(new RelayProperties());
        Tunnel tunnel = Tunnel.builder()
                .tunnelId("aaaadysa")
                .tunnelCode(123456L)
                .namespace("ns-user-001")
                .clusterId("cluster-a")
                .deleted(0)
                .build();

        when(tunnelRepository.findByTunnelIdAndRegionForUpdate("aaaadysa", "region-a")).thenReturn(tunnel);

        Boolean deleted = service.deleteTunnel("ns-user-001", "aaaadysa");

        assertThat(deleted).isTrue();
        verify(tunnelPortRepository).deleteByTunnelCode(123456L);
        verify(tunnelRepository).deleteByTunnelId("aaaadysa");
    }

    @Test
    void deleteTunnelsCleansLocalRegionTunnelsOnly() {
        TunnelAppService service = newService(new RelayProperties());
        Tunnel first = Tunnel.builder()
                .tunnelId("aaaadysa")
                .tunnelCode(123456L)
                .namespace("ns-user-001")
                .clusterId("cluster-a")
                .deleted(0)
                .build();

        when(tunnelRepository.findByNamespaceAndRegion("ns-user-001", "region-a")).thenReturn(List.of(first));
        when(tunnelRepository.findByTunnelIdAndRegionForUpdate("aaaadysa", "region-a")).thenReturn(first);

        Boolean deleted = service.deleteTunnels("ns-user-001");

        assertThat(deleted).isTrue();
        verify(tunnelPortRepository).deleteByTunnelCode(123456L);
        verify(tunnelRepository).deleteByTunnelId("aaaadysa");
    }

    @Test
    void issueTokenReturnsRequestedScopeAndLifetime() {
        TunnelAppService service = newService(new RelayProperties());
        Tunnel tunnel = Tunnel.builder()
                .tunnelId("aaaadysa")
                .namespace("ns-user-001")
                .expiration(Math.toIntExact(TimeUtils.nowSeconds() + 3600))
                .build();
        when(tunnelRepository.findByTunnelIdAndRegion("aaaadysa", "region-a")).thenReturn(tunnel);
        when(jwtTokenService.issueToken(tunnel, JwtScope.HOST, false))
                .thenReturn(new JwtToken("host-token", 3600L, 200000L));

        TunnelTokenResponse response = service.issueToken("ns-user-001", "aaaadysa", "host", false);

        assertThat(response.getTunnelId()).isEqualTo("aaaadysa");
        assertThat(response.getScope()).isEqualTo(JwtScope.HOST);
        assertThat(response.getLifetime()).isEqualTo(3600L);
        assertThat(response.getExpiration()).isEqualTo(200000L);
        assertThat(response.getToken()).isEqualTo("host-token");
        verify(billingService).assertTrafficAllowed("ns-user-001");
        verify(tunnelRepository, never()).refreshExpiration(anyString(), anyString(), anyLong());
    }

    @Test
    void issueTokenRejectsUnsupportedScope() {
        TunnelAppService service = newService(new RelayProperties());

        assertThatThrownBy(() -> service.issueToken("ns-user-001", "aaaadysa", "admin", false))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.PARAM_INVALID);
    }

    private TunnelAppService newService(RelayProperties properties) {
        org.mockito.Mockito.lenient()
                .when(billingService.lockAccountForQuota("ns-user-001"))
                .thenReturn(accountPlan());
        return new TunnelAppService(
                tunnelRepository,
                new LocalClusterService(clusterRepository, properties),
                new NamespaceService(),
                new FixedTunnelCodeGenerator(),
                jwtTokenService,
                new TunnelDomainService(),
                tunnelPortRepository,
                tunnelRuntimeStatusRepository,
                properties,
                billingService);
    }

    private static class FixedTunnelCodeGenerator extends TunnelCodeGenerator {
        @Override
        public long generate() {
            return 123456L;
        }
    }

    private static AccountPlan accountPlan() {
        return new AccountPlan(
                BillingAccount.builder().id(7L).namespace("ns-user-001").planCode("trial").status("active").build(),
                BillingPlan.builder().planCode("trial").maxTunnels(10).maxPortsPerTunnel(10).build());
    }
}
