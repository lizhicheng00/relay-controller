package com.huawei.devbridge.relaycontroller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.JwtScope;
import com.huawei.devbridge.relaycontroller.domain.model.JwtToken;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import com.huawei.devbridge.relaycontroller.infrastructure.security.JwtSigner;
import com.huawei.devbridge.relaycontroller.infrastructure.security.JwtTokenServiceImpl;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
class JwtTokenServiceTest {
    @Mock
    private JwtSigner jwtSigner;

    @Test
    void issueTokenCreatesOnlyRequestedScope() {
        RelayProperties properties = new RelayProperties();
        properties.getJwt().getToken().setTtlSeconds(86400);
        JwtTokenServiceImpl service = new JwtTokenServiceImpl(jwtSigner, properties);
        Tunnel tunnel = Tunnel.builder().tunnelId("aaaadysa").build();
        when(jwtSigner.signToken(eq(tunnel), eq(JwtScope.HOST), anyLong(), anyLong()))
                .thenReturn("host-token");

        JwtToken token = service.issueToken(tunnel, JwtScope.HOST);

        assertThat(token.token()).isEqualTo("host-token");
        assertThat(token.lifetime()).isEqualTo(86400L);
        assertThat(token.expiration() - token.lifetime()).isCloseTo(TimeUtils.nowSeconds(),
                org.assertj.core.data.Offset.offset(1L));
        verify(jwtSigner).signToken(tunnel, JwtScope.HOST,
                token.expiration() - token.lifetime(), token.expiration());
    }

    @Test
    void issueTokenUsesConfiguredLifetimeRegardlessOfTunnelExpiration() {
        RelayProperties properties = new RelayProperties();
        properties.getJwt().getToken().setTtlSeconds(86400);
        JwtTokenServiceImpl service = new JwtTokenServiceImpl(jwtSigner, properties);
        int tunnelExpiration = Math.toIntExact(TimeUtils.nowSeconds() + 60);
        Tunnel tunnel = Tunnel.builder().tunnelId("aaaadysa").expiration(tunnelExpiration).build();
        when(jwtSigner.signToken(eq(tunnel), eq(JwtScope.CONNECT), anyLong(), anyLong()))
                .thenReturn("connect-token");

        JwtToken token = service.issueToken(tunnel, JwtScope.CONNECT);

        assertThat(token.token()).isEqualTo("connect-token");
        assertThat(token.lifetime()).isEqualTo(86400L);
        assertThat(token.expiration()).isGreaterThan(tunnelExpiration);
    }

    @Test
    void issueTokenRejectsNonPositiveConfiguredLifetime() {
        RelayProperties properties = new RelayProperties();
        properties.getJwt().getToken().setTtlSeconds(0);
        JwtTokenServiceImpl service = new JwtTokenServiceImpl(jwtSigner, properties);

        assertThatThrownBy(() -> service.issueToken(Tunnel.builder().build(), JwtScope.HOST))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.JWT_GENERATE_FAILED);
    }

    @Test
    void issueAuthTokenUsesIndependentLifetime() {
        RelayProperties properties = new RelayProperties();
        properties.getJwt().getToken().setTtlSeconds(86400);
        properties.getJwt().getAuthToken().setTtlSeconds(3600);
        JwtTokenServiceImpl service = new JwtTokenServiceImpl(jwtSigner, properties);
        when(jwtSigner.signAuthToken(eq("ns-1"), anyLong(), anyLong()))
                .thenReturn("auth-token");

        JwtToken token = service.issueAuthToken("ns-1");

        assertThat(token.token()).isEqualTo("auth-token");
        assertThat(token.lifetime()).isEqualTo(3600L);
        verify(jwtSigner).signAuthToken(
                "ns-1", token.expiration() - token.lifetime(), token.expiration());
    }

    @Test
    void issueAuthTokenRejectsNonPositiveConfiguredLifetime() {
        RelayProperties properties = new RelayProperties();
        properties.getJwt().getAuthToken().setTtlSeconds(0);
        JwtTokenServiceImpl service = new JwtTokenServiceImpl(jwtSigner, properties);

        assertThatThrownBy(() -> service.issueAuthToken("ns-1"))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.JWT_GENERATE_FAILED);
    }
}
