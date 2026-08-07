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
    public CreateTunnelResponse createTunnel(
            String xNamespace, String xAccountNamespace, CreateTunnelRequest request) {
        return tunnelAppService.createTunnel(xNamespace, xAccountNamespace, request);
    }

    @Override
    public Boolean deleteTunnel(String xNamespace, String xAccountNamespace, String tunnelId) {
        return tunnelAppService.deleteTunnel(xNamespace, xAccountNamespace, tunnelId);
    }

    @Override
    public Boolean deleteTunnels(String xNamespace, String xAccountNamespace) {
        return tunnelAppService.deleteTunnels(xNamespace, xAccountNamespace);
    }

    @Override
    public TunnelDetailResponse getTunnelDetail(String xNamespace, String xAccountNamespace, String tunnelId) {
        return tunnelAppService.getTunnelDetail(xNamespace, xAccountNamespace, tunnelId);
    }

    @Override
    public TunnelTokenResponse issueTunnelToken(
            String xNamespace, String xAccountNamespace, String tunnelId, String scope) {
        return tunnelAppService.issueToken(xNamespace, xAccountNamespace, tunnelId, scope);
    }

    @Override
    public List<TunnelListItemResponse> listTunnels(
            String xNamespace, String xAccountNamespace, String clusterId) {
        return tunnelAppService.listTunnels(xNamespace, xAccountNamespace, clusterId);
    }

    @Override
    public Boolean updateTunnel(
            String xNamespace, String xAccountNamespace, String tunnelId, UpdateTunnelRequest request) {
        return tunnelAppService.updateTunnel(xNamespace, xAccountNamespace, tunnelId, request);
    }
}
