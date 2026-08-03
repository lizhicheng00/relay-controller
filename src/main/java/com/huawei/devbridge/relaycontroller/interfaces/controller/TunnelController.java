package com.huawei.devbridge.relaycontroller.interfaces.controller;

import com.huawei.devbridge.relaycontroller.application.service.TunnelAppService;
import com.huawei.devbridge.relaycontroller.generated.api.TunnelApi;
import com.huawei.devbridge.relaycontroller.interfaces.request.CreateTunnelRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.UpdateTunnelRequest;
import com.huawei.devbridge.relaycontroller.interfaces.response.CreateTunnelResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelDetailResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelListItemResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelTokenResponse;
import java.util.List;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class TunnelController implements TunnelApi {
    private final TunnelAppService tunnelAppService;

    @Override
    public CreateTunnelResponse createTunnel(String xNamespace, CreateTunnelRequest request) {
        return tunnelAppService.createTunnel(xNamespace, request);
    }

    @Override
    public Boolean deleteTunnel(String xNamespace, String tunnelId) {
        return tunnelAppService.deleteTunnel(xNamespace, tunnelId);
    }

    @Override
    public Boolean deleteTunnels(String xNamespace) {
        return tunnelAppService.deleteTunnels(xNamespace);
    }

    @Override
    public TunnelDetailResponse getTunnelDetail(String xNamespace, String tunnelId) {
        return tunnelAppService.getTunnelDetail(xNamespace, tunnelId);
    }

    @Override
    public TunnelTokenResponse issueTunnelToken(String xNamespace, String tunnelId, String scope) {
        return tunnelAppService.issueToken(xNamespace, tunnelId, scope);
    }

    @Override
    public List<TunnelListItemResponse> listTunnels(String xNamespace, String clusterId) {
        return tunnelAppService.listTunnels(xNamespace, clusterId);
    }

    @Override
    public Boolean updateTunnel(String xNamespace, String tunnelId, UpdateTunnelRequest request) {
        return tunnelAppService.updateTunnel(xNamespace, tunnelId, request);
    }
}
