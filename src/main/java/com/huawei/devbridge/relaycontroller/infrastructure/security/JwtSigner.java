package com.huawei.devbridge.relaycontroller.infrastructure.security;

import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.domain.model.JwtScope;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import com.nimbusds.jose.JOSEObjectType;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import java.security.interfaces.RSAPrivateKey;
import java.time.Instant;
import java.util.Date;
import java.util.UUID;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class JwtSigner {
    private final RelayProperties relayProperties;
    private final JwtKeyProvider jwtKeyProvider;

    public String signToken(Tunnel tunnel, JwtScope scope, long issuedAt, long expiration) {
        JWTClaimsSet claims = claims(relayProperties.getJwt().getAudience(), issuedAt, expiration)
                .claim("tunnelId", tunnel.getTunnelId())
                .claim("clusterId", tunnel.getClusterId())
                .claim("scp", scope.value())
                .build();
        return sign(claims);
    }

    private JWTClaimsSet.Builder claims(String audience, long issuedAt, long expiration) {
        return new JWTClaimsSet.Builder()
                .issuer(relayProperties.getJwt().getIssuer())
                .audience(audience)
                .expirationTime(Date.from(Instant.ofEpochSecond(expiration)))
                .notBeforeTime(Date.from(Instant.ofEpochSecond(issuedAt)))
                .jwtID(UUID.randomUUID().toString());
    }

    private String sign(JWTClaimsSet claims) {
        try {
            JWSHeader header = new JWSHeader.Builder(JWSAlgorithm.RS256)
                    .type(JOSEObjectType.JWT)
                    .keyID(relayProperties.getJwt().getKeyId())
                    .build();
            SignedJWT jwt = new SignedJWT(header, claims);
            jwt.sign(new RSASSASigner((RSAPrivateKey) jwtKeyProvider.getPrivateKey()));
            return jwt.serialize();
        } catch (Exception exception) {
            throw new BizException(ErrorCode.JWT_GENERATE_FAILED);
        }
    }
}
