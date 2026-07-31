package com.huawei.devbridge.relaycontroller.infrastructure.security;

import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.JwtScope;
import com.huawei.devbridge.relaycontroller.domain.model.JwtToken;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.service.JwtTokenService;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class JwtTokenServiceImpl implements JwtTokenService {
    private final JwtSigner jwtSigner;
    private final RelayProperties relayProperties;

    @Override
    public JwtToken issueToken(Tunnel tunnel, JwtScope scope, boolean forCookies) {
        long lifetime = relayProperties.getJwt().getToken().getTtlSeconds();
        return issue(lifetime,
                (issuedAt, expiration) -> jwtSigner.signToken(
                        tunnel, scope, issuedAt, expiration, forCookies));
    }

    @Override
    public JwtToken issueAuthToken(String namespace) {
        long lifetime = relayProperties.getJwt().getAuthToken().getTtlSeconds();
        return issue(lifetime,
                (issuedAt, expiration) -> jwtSigner.signAuthToken(namespace, issuedAt, expiration));
    }

    private JwtToken issue(long lifetime, TokenFactory tokenFactory) {
        if (lifetime <= 0) {
            throw new BizException(ErrorCode.JWT_GENERATE_FAILED);
        }
        long issuedAt = TimeUtils.nowSeconds();
        long expiration;
        try {
            expiration = Math.addExact(issuedAt, lifetime);
        } catch (ArithmeticException exception) {
            throw new BizException(ErrorCode.JWT_GENERATE_FAILED);
        }
        return new JwtToken(tokenFactory.create(issuedAt, expiration), lifetime, expiration);
    }

    @FunctionalInterface
    private interface TokenFactory {
        String create(long issuedAt, long expiration);
    }
}
