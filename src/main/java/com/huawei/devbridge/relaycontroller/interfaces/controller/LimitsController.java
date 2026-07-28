package com.huawei.devbridge.relaycontroller.interfaces.controller;

import com.huawei.devbridge.relaycontroller.application.service.LimitsAppService;
import com.huawei.devbridge.relaycontroller.generated.api.LimitsApi;
import com.huawei.devbridge.relaycontroller.interfaces.response.LimitsResponse;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class LimitsController implements LimitsApi {
    private final LimitsAppService limitsAppService;

    @Override
    public LimitsResponse getLimits(String xNamespace) {
        return limitsAppService.getLimits(xNamespace);
    }
}
