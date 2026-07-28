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
        long issuedAt = TimeUtils.nowSeconds();
        long lifetime = relayProperties.getJwt().getToken().getTtlSeconds();
        if (lifetime <= 0) {
            throw new BizException(ErrorCode.JWT_GENERATE_FAILED);
        }
        long expiration = issuedAt + lifetime;
        String token = jwtSigner.signToken(tunnel, scope, issuedAt, expiration, forCookies);
        return new JwtToken(token, lifetime, expiration);
    }
}
