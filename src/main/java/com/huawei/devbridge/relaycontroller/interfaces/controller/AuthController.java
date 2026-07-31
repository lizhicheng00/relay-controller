package com.huawei.devbridge.relaycontroller.interfaces.controller;

import com.huawei.devbridge.relaycontroller.application.service.AuthAppService;
import com.huawei.devbridge.relaycontroller.generated.api.AuthApi;
import com.huawei.devbridge.relaycontroller.interfaces.response.AuthTokenResponse;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class AuthController implements AuthApi {
    private final AuthAppService authAppService;

    @Override
    public AuthTokenResponse issueAuthToken(String xNamespace) {
        return authAppService.issueToken(xNamespace);
    }
}
