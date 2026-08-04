package com.huawei.devbridge.relaycontroller.application.assembler;

import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelPort;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelPortResponse;

public final class TunnelPortAssembler {
    private TunnelPortAssembler() {
    }

    public static TunnelPortResponse toResponse(Tunnel tunnel, TunnelPort tunnelPort) {
        return TunnelPortResponse.builder()
                .tunnelId(tunnel.getTunnelId())
                .tunnelCode(tunnelPort.getTunnelCode())
                .port(tunnelPort.getPort())
                .protocol(tunnelPort.getProtocol())
                .allowAnonymous(tunnelPort.getAllowAnonymous())
                .build();
    }
}
