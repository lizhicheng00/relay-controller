package com.huawei.devbridge.relaycontroller.interfaces.controller;

import com.huawei.devbridge.relaycontroller.application.service.TunnelStatusAppService;
import com.huawei.devbridge.relaycontroller.generated.api.GatewayTunnelStatusApi;
import com.huawei.devbridge.relaycontroller.interfaces.request.TunnelStatusReportRequest;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelStatusReportResponse;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class TunnelStatusController implements GatewayTunnelStatusApi {
    private final TunnelStatusAppService tunnelStatusAppService;

    @Override
    public TunnelStatusReportResponse reportTunnelStatus(
            String clusterId, TunnelStatusReportRequest request) {
        return tunnelStatusAppService.report(clusterId, request);
    }
}
