package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.domain.model.JwtToken;
import com.huawei.devbridge.relaycontroller.domain.service.JwtTokenService;
import com.huawei.devbridge.relaycontroller.domain.service.NamespaceService;
import com.huawei.devbridge.relaycontroller.interfaces.response.AuthTokenResponse;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class AuthAppService {
    private static final String AUTH_SCOPE = "devbridge";

    private final NamespaceService namespaceService;
    private final JwtTokenService jwtTokenService;

    public AuthTokenResponse issueToken(String rawNamespace) {
        String namespace = namespaceService.requireNamespace(rawNamespace);
        JwtToken issuedToken = jwtTokenService.issueAuthToken(namespace);
        return AuthTokenResponse.builder()
                .namespace(namespace)
                .scope(AUTH_SCOPE)
                .lifetime(issuedToken.lifetime())
                .expiration(issuedToken.expiration())
                .token(issuedToken.token())
                .build();
    }
}
