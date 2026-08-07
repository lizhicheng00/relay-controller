package com.huawei.devbridge.relaycontroller.interfaces.controller;

import com.huawei.devbridge.relaycontroller.application.service.TunnelPortAppService;
import com.huawei.devbridge.relaycontroller.generated.api.TunnelPortApi;
import com.huawei.devbridge.relaycontroller.interfaces.request.CreateTunnelPortRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.UpdateTunnelPortRequest;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelPortResponse;
import java.util.List;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class TunnelPortController implements TunnelPortApi {
    private final TunnelPortAppService tunnelPortAppService;

    @Override
    public TunnelPortResponse createTunnelPort(
            String xNamespace, String xAccountNamespace, String tunnelId, CreateTunnelPortRequest request) {
        return tunnelPortAppService.create(xNamespace, xAccountNamespace, tunnelId, request);
    }

    @Override
    public Boolean deleteTunnelPort(String xNamespace, String xAccountNamespace, String tunnelId, Long port) {
        return tunnelPortAppService.delete(xNamespace, xAccountNamespace, tunnelId, port);
    }

    @Override
    public TunnelPortResponse getTunnelPort(
            String xNamespace, String xAccountNamespace, String tunnelId, Long port) {
        return tunnelPortAppService.detail(xNamespace, xAccountNamespace, tunnelId, port);
    }

    @Override
    public List<TunnelPortResponse> listTunnelPorts(
            String xNamespace, String xAccountNamespace, String tunnelId) {
        return tunnelPortAppService.list(xNamespace, xAccountNamespace, tunnelId);
    }

    @Override
    public TunnelPortResponse updateTunnelPort(
            String xNamespace, String xAccountNamespace, String tunnelId, Long port,
            UpdateTunnelPortRequest request) {
        return tunnelPortAppService.update(xNamespace, xAccountNamespace, tunnelId, port, request);
    }
}
