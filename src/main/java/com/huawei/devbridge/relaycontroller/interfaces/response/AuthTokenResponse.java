package com.huawei.devbridge.relaycontroller.interfaces.response;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class AuthTokenResponse {
    private String namespace;
    private String scope;
    private Long lifetime;
    private Long expiration;
    private String token;
}
