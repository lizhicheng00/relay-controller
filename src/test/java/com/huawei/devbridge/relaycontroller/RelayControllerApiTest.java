package com.huawei.devbridge.relaycontroller;

import static org.hamcrest.Matchers.hasSize;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.huawei.devbridge.relaycontroller.application.service.MeteringAppService;
import com.huawei.devbridge.relaycontroller.application.service.LimitsAppService;
import com.huawei.devbridge.relaycontroller.application.service.TunnelAppService;
import com.huawei.devbridge.relaycontroller.application.service.TunnelPortAppService;
import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.exception.GlobalExceptionHandler;
import com.huawei.devbridge.relaycontroller.interfaces.controller.MeteringController;
import com.huawei.devbridge.relaycontroller.interfaces.controller.LimitsController;
import com.huawei.devbridge.relaycontroller.interfaces.controller.TunnelController;
import com.huawei.devbridge.relaycontroller.interfaces.controller.TunnelPortController;
import com.huawei.devbridge.relaycontroller.interfaces.config.TokenCacheControlAdvice;
import com.huawei.devbridge.relaycontroller.interfaces.request.CreateTunnelPortRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.CreateTunnelRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.MeteringReportRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.UpdateTunnelPortRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.UpdateTunnelRequest;
import com.huawei.devbridge.relaycontroller.interfaces.response.CreateTunnelResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.GatewayTunnelPortPolicyResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.LimitsResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.MeteringReportResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelDetailResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelListItemResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelPortResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelTokenResponse;
import com.huawei.devbridge.relaycontroller.domain.model.JwtScope;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelProtocol;
import java.util.List;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.http.MediaType;
import org.springframework.http.converter.json.MappingJackson2HttpMessageConverter;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(MockitoExtension.class)
class RelayControllerApiTest {
    private static final String BASE = "/open-api-inner/v1/relay-controller";
    private static final String NAMESPACE = "ns-user-001";
    private static final String TUNNEL_ID = "aaaadysa";
    private static final String CLUSTER_ID = "cluster-a";

    private MockMvc mockMvc;

    @Mock
    private TunnelAppService tunnelAppService;
    @Mock
    private MeteringAppService meteringAppService;
    @Mock
    private TunnelPortAppService tunnelPortAppService;
    @Mock
    private LimitsAppService limitsAppService;

    @BeforeEach
    void setUp() {
        mockMvc = MockMvcBuilders.standaloneSetup(
                        new TunnelController(tunnelAppService),
                        new MeteringController(meteringAppService),
                        new TunnelPortController(tunnelPortAppService),
                        new LimitsController(limitsAppService))
                .setControllerAdvice(new GlobalExceptionHandler(), new TokenCacheControlAdvice())
                .setMessageConverters(new MappingJackson2HttpMessageConverter())
                .defaultRequest(get("/").accept(MediaType.APPLICATION_JSON))
                .build();
    }

    @Test
    void createTunnelApi() throws Exception {
        when(tunnelAppService.createTunnel(eq(NAMESPACE), any(CreateTunnelRequest.class)))
                .thenReturn(createTunnelResponse());

        mockMvc.perform(post(BASE + "/tunnels")
                        .header("X-Namespace", NAMESPACE)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "name": "dev",
                                  "clusterId": "cluster-a",
                                  "type": "bridge"
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.tunnelId").value(TUNNEL_ID))
                .andExpect(jsonPath("$.clusterId").value(CLUSTER_ID))
                .andExpect(jsonPath("$.expirationHours").value(720))
                .andExpect(jsonPath("$.tunnelExpiration").value(1722592000L))
                .andExpect(jsonPath("$.expiresAt").doesNotExist())
                .andExpect(jsonPath("$.jwt").doesNotExist())
                .andExpect(jsonPath("$.data").doesNotExist())
                .andExpect(jsonPath("$.error_code").doesNotExist());
    }

    @Test
    void createTunnelWithoutUserHeaderReturnsUnauthorized() throws Exception {
        mockMvc.perform(post(BASE + "/tunnels")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "name": "dev",
                                  "clusterId": "cluster-a",
                                  "type": "bridge"
                                }
                                """))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.error.code").value("40100"))
                .andExpect(jsonPath("$.error.message").value("X-Namespace is required"))
                .andExpect(jsonPath("$.error.target").value("X-Namespace"))
                .andExpect(jsonPath("$.error_code").doesNotExist());
    }

    @Test
    void createTunnelWithInvalidTypeReturnsParamInvalid() throws Exception {
        mockMvc.perform(post(BASE + "/tunnels")
                        .header("X-Namespace", NAMESPACE)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "name": "dev",
                                  "clusterId": "cluster-a",
                                  "type": "default"
                                }
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.error.code").value("40000"))
                .andExpect(jsonPath("$.error.target").value("requestBody"));
    }

    @Test
    void createTunnelWithTooLargeExpirationReturnsParamInvalid() throws Exception {
        mockMvc.perform(post(BASE + "/tunnels")
                        .header("X-Namespace", NAMESPACE)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "name": "dev",
                                  "clusterId": "cluster-a",
                                  "type": "bridge",
                                  "expiration": 721
                                }
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.error.code").value("40000"))
                .andExpect(jsonPath("$.error.details[0].target").value("expiration"));
    }

    @Test
    void getTunnelDetailNotFoundReturns404() throws Exception {
        when(tunnelAppService.getTunnelDetail(NAMESPACE, TUNNEL_ID))
                .thenThrow(new BizException(ErrorCode.TUNNEL_NOT_FOUND));

        mockMvc.perform(get(BASE + "/tunnels/{tunnelId}", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.error.code").value("10002"));
    }

    @Test
    void getTunnelDetailUnexpectedErrorReturns500() throws Exception {
        when(tunnelAppService.getTunnelDetail(NAMESPACE, TUNNEL_ID))
                .thenThrow(new IllegalStateException("database password leaked"));

        mockMvc.perform(get(BASE + "/tunnels/{tunnelId}", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE))
                .andExpect(status().isInternalServerError())
                .andExpect(jsonPath("$.error.code").value("50000"))
                .andExpect(jsonPath("$.error.message").value("internal error"))
                .andExpect(jsonPath("$.error.target").doesNotExist())
                .andExpect(jsonPath("$.error.innerError").doesNotExist());
    }

    @Test
    void listTunnelsApi() throws Exception {
        when(tunnelAppService.listTunnels(NAMESPACE, CLUSTER_ID)).thenReturn(List.of(
                TunnelListItemResponse.builder()
                        .tunnelId(TUNNEL_ID)
                        .tunnelCode(123456L)
                        .clusterId(CLUSTER_ID)
                        .name("dev")
                        .description("dev tunnel")
                        .expirationHours(720)
                        .tunnelExpiration(1722592000L)
                        .created(1720000000L)
                        .url("aaaadysa.cluster-a.myhuaweicloud.com")
                        .portCount(2L)
                        .build()));

        mockMvc.perform(get(BASE + "/tunnels")
                        .header("X-Namespace", NAMESPACE)
                        .param("clusterId", CLUSTER_ID))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$", hasSize(1)))
                .andExpect(jsonPath("$[0].tunnelId").value(TUNNEL_ID))
                .andExpect(jsonPath("$[0].clusterId").value(CLUSTER_ID))
                .andExpect(jsonPath("$[0].name").value("dev"))
                .andExpect(jsonPath("$[0].expirationHours").value(720))
                .andExpect(jsonPath("$[0].tunnelExpiration").value(1722592000L))
                .andExpect(jsonPath("$[0].expiresAt").doesNotExist())
                .andExpect(jsonPath("$[0].portCount").value(2));
    }

    @Test
    void getTunnelDetailApi() throws Exception {
        when(tunnelAppService.getTunnelDetail(NAMESPACE, TUNNEL_ID)).thenReturn(TunnelDetailResponse.builder()
                .name("dev")
                .tunnelId(TUNNEL_ID)
                .tunnelCode(123456L)
                .clusterId(CLUSTER_ID)
                .expirationHours(720)
                .tunnelExpiration(1722592000L)
                .url("aaaadysa.cluster-a.myhuaweicloud.com")
                .type("bridge")
                .hostConnections(1)
                .clientConnections(2)
                .channelCount(3)
                .uploadBytesPerSecond(1024L)
                .downloadBytesPerSecond(2048L)
                .statusReportedAt(1720000000L)
                .build());

        mockMvc.perform(get(BASE + "/tunnels/{tunnelId}", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.tunnelId").value(TUNNEL_ID))
                .andExpect(jsonPath("$.clusterId").value(CLUSTER_ID))
                .andExpect(jsonPath("$.expirationHours").value(720))
                .andExpect(jsonPath("$.tunnelExpiration").value(1722592000L))
                .andExpect(jsonPath("$.hostConnections").value(1))
                .andExpect(jsonPath("$.clientConnections").value(2))
                .andExpect(jsonPath("$.channelCount").value(3))
                .andExpect(jsonPath("$.uploadBytesPerSecond").value(1024L))
                .andExpect(jsonPath("$.downloadBytesPerSecond").value(2048L))
                .andExpect(jsonPath("$.statusReportedAt").value(1720000000L))
                .andExpect(jsonPath("$.expiresAt").doesNotExist())
                .andExpect(jsonPath("$.jwt").doesNotExist());
    }

    @Test
    void issueTunnelTokenApi() throws Exception {
        when(tunnelAppService.issueToken(NAMESPACE, TUNNEL_ID, "host", false))
                .thenReturn(TunnelTokenResponse.builder()
                        .tunnelId(TUNNEL_ID)
                        .scope(JwtScope.HOST)
                        .lifetime(3600L)
                        .expiration(1720086400L)
                        .token("host-token")
                        .build());

        mockMvc.perform(post(BASE + "/tunnels/{tunnelId}/token", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE)
                        .param("scope", "host"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.tunnelId").value(TUNNEL_ID))
                .andExpect(jsonPath("$.scope").value("host"))
                .andExpect(jsonPath("$.lifetime").value(3600))
                .andExpect(jsonPath("$.expiration").value(1720086400L))
                .andExpect(jsonPath("$.token").value("host-token"))
                .andExpect(header().string("Cache-Control", "no-store"));
    }

    @Test
    void issueCookieTunnelTokenApi() throws Exception {
        when(tunnelAppService.issueToken(NAMESPACE, TUNNEL_ID, "connect", true))
                .thenReturn(TunnelTokenResponse.builder()
                        .tunnelId(TUNNEL_ID)
                        .scope(JwtScope.CONNECT)
                        .lifetime(3600L)
                        .expiration(1720086400L)
                        .token("cookie-token")
                        .build());

        mockMvc.perform(post(BASE + "/tunnels/{tunnelId}/token", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE)
                        .param("scope", "connect")
                        .param("forCookies", "true"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.scope").value("connect"))
                .andExpect(jsonPath("$.token").value("cookie-token"))
                .andExpect(header().string("Cache-Control", "no-store"));
    }

    @Test
    void getLimitsApi() throws Exception {
        when(limitsAppService.getLimits(NAMESPACE)).thenReturn(LimitsResponse.builder()
                .namespace(NAMESPACE)
                .planCode("trial")
                .resetAt(1785542400L)
                .quotaBytes(5368709120L)
                .usedBytes(1536L)
                .remainingBytes(5368707584L)
                .exhausted(false)
                .activeTunnels(2L)
                .maxTunnels(10)
                .maxPortsPerTunnel(10)
                .maxHostsPerTunnel(1)
                .maxTunnelBandwidthBytesPerSecond(5242880L)
                .maxHttpRequestsPerMinutePerPort(500)
                .maxConnectionsPerPort(100)
                .build());

        mockMvc.perform(get(BASE + "/limits").header("X-Namespace", NAMESPACE))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.planCode").value("trial"))
                .andExpect(jsonPath("$.resetAt").value(1785542400L))
                .andExpect(jsonPath("$.quotaBytes").value(5368709120L))
                .andExpect(jsonPath("$.usedBytes").value(1536L))
                .andExpect(jsonPath("$.remainingBytes").value(5368707584L))
                .andExpect(jsonPath("$.billedBytes").doesNotExist())
                .andExpect(jsonPath("$.pendingBytes").doesNotExist())
                .andExpect(jsonPath("$.maxTunnels").value(10))
                .andExpect(jsonPath("$.maxConnectionsPerPort").value(100));
    }

    @Test
    void issueTunnelTokenWithInvalidScopeReturnsBadRequest() throws Exception {
        when(tunnelAppService.issueToken(NAMESPACE, TUNNEL_ID, "admin", false))
                .thenThrow(new BizException(ErrorCode.PARAM_INVALID, "scope must be host or connect"));

        mockMvc.perform(post(BASE + "/tunnels/{tunnelId}/token", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE)
                        .param("scope", "admin"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.error.code").value("40000"));
    }

    @Test
    void issueTunnelTokenWithoutScopeReturnsBadRequest() throws Exception {
        mockMvc.perform(post(BASE + "/tunnels/{tunnelId}/token", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.error.code").value("40000"))
                .andExpect(jsonPath("$.error.target").value("scope"));
    }

    @Test
    void updateTunnelApi() throws Exception {
        when(tunnelAppService.updateTunnel(eq(NAMESPACE), eq(TUNNEL_ID), any(UpdateTunnelRequest.class)))
                .thenReturn(true);

        mockMvc.perform(put(BASE + "/tunnels/{tunnelId}", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "name": "dev-renamed"
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$").value(true));
    }

    @Test
    void deleteTunnelApi() throws Exception {
        when(tunnelAppService.deleteTunnel(NAMESPACE, TUNNEL_ID)).thenReturn(true);

        mockMvc.perform(delete(BASE + "/tunnels/{tunnelId}", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$").value(true));
    }

    @Test
    void deleteTunnelsApi() throws Exception {
        when(tunnelAppService.deleteTunnels(NAMESPACE)).thenReturn(true);

        mockMvc.perform(delete(BASE + "/tunnels")
                        .header("X-Namespace", NAMESPACE))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$").value(true));
    }

    @Test
    void reportMeteringApi() throws Exception {
        when(meteringAppService.report(eq(CLUSTER_ID), any(MeteringReportRequest.class)))
                .thenReturn(MeteringReportResponse.builder().accepted(true).build());

        mockMvc.perform(post(BASE + "/clusters/{clusterId}/metering", CLUSTER_ID)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "tunnelCode": 123456,
                                  "tunnelId": "aaaadysa",
                                  "usage": 1024
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.accepted").value(true));
    }

    @Test
    void reportMeteringWithInvalidTunnelIdReturnsParamInvalid() throws Exception {
        mockMvc.perform(post(BASE + "/clusters/{clusterId}/metering", CLUSTER_ID)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "tunnelCode": 123456,
                                  "tunnelId": "not-valid",
                                  "usage": 1024
                                }
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.error.code").value("40000"))
                .andExpect(jsonPath("$.error.details[0].target").value("tunnelId"));
    }

    @Test
    void createTunnelPortApi() throws Exception {
        when(tunnelPortAppService.create(eq(NAMESPACE), eq(TUNNEL_ID), any(CreateTunnelPortRequest.class)))
                .thenReturn(tunnelPortResponse(false));

        mockMvc.perform(post(BASE + "/tunnels/{tunnelId}/ports", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "port": 8080,
                                  "protocol": "http",
                                  "allowAnonymous": false
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.port").value(8080))
                .andExpect(jsonPath("$.protocol").value("http"))
                .andExpect(jsonPath("$.allowAnonymous").value(false));
    }

    @Test
    void listTunnelPortsApi() throws Exception {
        when(tunnelPortAppService.list(NAMESPACE, TUNNEL_ID)).thenReturn(List.of(
                tunnelPortResponse(false),
                TunnelPortResponse.builder()
                        .tunnelId(TUNNEL_ID)
                        .tunnelCode(123456L)
                        .port(8888L)
                        .protocol(TunnelProtocol.HTTPS)
                        .allowAnonymous(true)
                        .build()));

        mockMvc.perform(get(BASE + "/tunnels/{tunnelId}/ports", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$", hasSize(2)))
                .andExpect(jsonPath("$[1].allowAnonymous").value(true));
    }

    @Test
    void getTunnelPortApi() throws Exception {
        when(tunnelPortAppService.detail(NAMESPACE, TUNNEL_ID, 8080L)).thenReturn(tunnelPortResponse(false));

        mockMvc.perform(get(BASE + "/tunnels/{tunnelId}/ports/{port}", TUNNEL_ID, 8080)
                        .header("X-Namespace", NAMESPACE))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.tunnelId").value(TUNNEL_ID))
                .andExpect(jsonPath("$.port").value(8080));
    }

    @Test
    void updateTunnelPortApi() throws Exception {
        when(tunnelPortAppService.update(eq(NAMESPACE), eq(TUNNEL_ID), eq(8080L), any(UpdateTunnelPortRequest.class)))
                .thenReturn(TunnelPortResponse.builder()
                        .tunnelId(TUNNEL_ID)
                        .tunnelCode(123456L)
                        .port(8080L)
                        .protocol(TunnelProtocol.HTTPS)
                        .allowAnonymous(false)
                        .build());

        mockMvc.perform(put(BASE + "/tunnels/{tunnelId}/ports/{port}", TUNNEL_ID, 8080)
                        .header("X-Namespace", NAMESPACE)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "protocol": "https"
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.protocol").value("https"))
                .andExpect(jsonPath("$.allowAnonymous").value(false));
    }

    @Test
    void createTunnelPortWithInvalidProtocolReturnsBadRequest() throws Exception {
        mockMvc.perform(post(BASE + "/tunnels/{tunnelId}/ports", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "port": 8080,
                                  "protocol": "tcp",
                                  "allowAnonymous": false
                                }
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.error.code").value("40000"));
    }

    @Test
    void tunnelPortCollectionDoesNotSupportDelete() throws Exception {
        mockMvc.perform(delete(BASE + "/tunnels/{tunnelId}/ports", TUNNEL_ID)
                        .header("X-Namespace", NAMESPACE))
                .andExpect(status().isMethodNotAllowed());
    }

    @Test
    void deleteTunnelPortApi() throws Exception {
        when(tunnelPortAppService.delete(NAMESPACE, TUNNEL_ID, 8080L)).thenReturn(true);

        mockMvc.perform(delete(BASE + "/tunnels/{tunnelId}/ports/{port}", TUNNEL_ID, 8080)
                        .header("X-Namespace", NAMESPACE))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$").value(true));
    }

    @Test
    void getGatewayTunnelPortPolicyApi() throws Exception {
        when(tunnelPortAppService.getGatewayPortPolicy(CLUSTER_ID, TUNNEL_ID, 8080L))
                .thenReturn(GatewayTunnelPortPolicyResponse.builder()
                        .tunnelId(TUNNEL_ID)
                        .tunnelCode(123456L)
                        .clusterId(CLUSTER_ID)
                        .port(8080L)
                        .protocol(TunnelProtocol.AUTO)
                        .allowAnonymous(false)
                        .build());

        mockMvc.perform(get(BASE + "/clusters/{clusterId}/tunnels/{tunnelId}/ports/{port}", CLUSTER_ID, TUNNEL_ID, 8080))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.clusterId").value(CLUSTER_ID))
                .andExpect(jsonPath("$.protocol").value("auto"))
                .andExpect(jsonPath("$.allowAnonymous").value(false));
    }

    private CreateTunnelResponse createTunnelResponse() {
        return CreateTunnelResponse.builder()
                .name("dev")
                .tunnelId(TUNNEL_ID)
                .tunnelCode(123456L)
                .clusterId(CLUSTER_ID)
                .bandwidthUsed(0L)
                .expirationHours(720)
                .tunnelExpiration(1722592000L)
                .created(1720000000L)
                .url("aaaadysa.cluster-a.myhuaweicloud.com")
                .type("bridge")
                .build();
    }

    private TunnelPortResponse tunnelPortResponse(boolean allowAnonymous) {
        return TunnelPortResponse.builder()
                .tunnelId(TUNNEL_ID)
                .tunnelCode(123456L)
                .port(8080L)
                .protocol(TunnelProtocol.HTTP)
                .allowAnonymous(allowAnonymous)
                .build();
    }
}
