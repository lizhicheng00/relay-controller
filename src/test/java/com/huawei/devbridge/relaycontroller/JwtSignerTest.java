package com.huawei.devbridge.relaycontroller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import com.huawei.devbridge.relaycontroller.domain.model.JwtScope;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import com.huawei.devbridge.relaycontroller.infrastructure.security.JwtKeyProvider;
import com.huawei.devbridge.relaycontroller.infrastructure.security.JwtSigner;
import com.nimbusds.jose.crypto.RSASSAVerifier;
import com.nimbusds.jwt.SignedJWT;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.interfaces.RSAPublicKey;
import java.time.Instant;
import java.util.Set;
import org.junit.jupiter.api.Test;

class JwtSignerTest {

    @Test
    void signsMinimalScopeSpecificClaims() throws Exception {
        KeyPairGenerator generator = KeyPairGenerator.getInstance("RSA");
        generator.initialize(2048);
        KeyPair keyPair = generator.generateKeyPair();
        JwtKeyProvider keyProvider = mock(JwtKeyProvider.class);
        when(keyProvider.getPrivateKey()).thenReturn(keyPair.getPrivate());
        RelayProperties properties = new RelayProperties();
        properties.getJwt().setIssuer("devbridge");
        properties.getJwt().setKeyId("1");
        JwtSigner signer = new JwtSigner(properties, keyProvider);
        Tunnel tunnel = Tunnel.builder().tunnelId("aaaadysa").clusterId("cluster-a").build();
        long issuedAt = Instant.now().getEpochSecond();
        long expiration = issuedAt + 3600;

        SignedJWT jwt = SignedJWT.parse(
                signer.signToken(tunnel, JwtScope.CONNECT, issuedAt, expiration, false));

        assertThat(jwt.verify(new RSASSAVerifier((RSAPublicKey) keyPair.getPublic()))).isTrue();
        assertThat(jwt.getHeader().getAlgorithm().getName()).isEqualTo("RS256");
        assertThat(jwt.getHeader().getType().getType()).isEqualTo("JWT");
        assertThat(jwt.getHeader().getKeyID()).isEqualTo("1");
        assertThat(jwt.getJWTClaimsSet().getClaims().keySet())
                .isEqualTo(Set.of(
                        "iss", "aud", "exp", "nbf", "jti", "tunnelId", "clusterId", "scp", "delivery"));
        assertThat(jwt.getJWTClaimsSet().getIssuer()).isEqualTo("devbridge");
        assertThat(jwt.getJWTClaimsSet().getAudience()).containsExactly("relay-gateway");
        assertThat(jwt.getJWTClaimsSet().getStringClaim("tunnelId")).isEqualTo("aaaadysa");
        assertThat(jwt.getJWTClaimsSet().getStringClaim("clusterId")).isEqualTo("cluster-a");
        assertThat(jwt.getJWTClaimsSet().getStringClaim("scp")).isEqualTo("connect");
        assertThat(jwt.getJWTClaimsSet().getStringClaim("delivery")).isEqualTo("api");
        assertThat(jwt.getJWTClaimsSet().getJWTID()).isNotBlank();
        assertThat(jwt.getJWTClaimsSet().getNotBeforeTime().toInstant().getEpochSecond()).isEqualTo(issuedAt);
        assertThat(jwt.getJWTClaimsSet().getExpirationTime().toInstant().getEpochSecond()).isEqualTo(expiration);
    }

    @Test
    void marksCookieDeliveryWithoutChangingTokenSigning() throws Exception {
        KeyPairGenerator generator = KeyPairGenerator.getInstance("RSA");
        generator.initialize(2048);
        KeyPair keyPair = generator.generateKeyPair();
        JwtKeyProvider keyProvider = mock(JwtKeyProvider.class);
        when(keyProvider.getPrivateKey()).thenReturn(keyPair.getPrivate());
        JwtSigner signer = new JwtSigner(new RelayProperties(), keyProvider);
        Tunnel tunnel = Tunnel.builder().tunnelId("aaaadysa").clusterId("cluster-a").build();
        long issuedAt = Instant.now().getEpochSecond();

        SignedJWT jwt = SignedJWT.parse(
                signer.signToken(tunnel, JwtScope.CONNECT, issuedAt, issuedAt + 3600, true));

        assertThat(jwt.verify(new RSASSAVerifier((RSAPublicKey) keyPair.getPublic()))).isTrue();
        assertThat(jwt.getJWTClaimsSet().getStringClaim("delivery")).isEqualTo("cookie");
    }

    @Test
    void eachCallCreatesANewToken() throws Exception {
        KeyPairGenerator generator = KeyPairGenerator.getInstance("RSA");
        generator.initialize(2048);
        KeyPair keyPair = generator.generateKeyPair();
        JwtKeyProvider keyProvider = mock(JwtKeyProvider.class);
        when(keyProvider.getPrivateKey()).thenReturn(keyPair.getPrivate());
        JwtSigner signer = new JwtSigner(new RelayProperties(), keyProvider);
        Tunnel tunnel = Tunnel.builder().tunnelId("aaaadysa").clusterId("cluster-a").build();
        long issuedAt = Instant.now().getEpochSecond();

        String first = signer.signToken(tunnel, JwtScope.CONNECT, issuedAt, issuedAt + 3600, false);
        String second = signer.signToken(tunnel, JwtScope.CONNECT, issuedAt, issuedAt + 3600, false);

        assertThat(first).isNotEqualTo(second);
    }
}
