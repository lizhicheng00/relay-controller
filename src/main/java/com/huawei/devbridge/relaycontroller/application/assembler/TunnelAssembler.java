package com.huawei.devbridge.relaycontroller.application.assembler;

import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelRuntimeStatus;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelType;
import com.huawei.devbridge.relaycontroller.interfaces.response.CreateTunnelResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelDetailResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelListItemResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelStatusResponse;

public final class TunnelAssembler {
    private TunnelAssembler() {
    }

    public static CreateTunnelResponse toCreateResponse(Tunnel tunnel) {
        return CreateTunnelResponse.builder()
                .name(tunnel.getName())
                .tunnelId(tunnel.getTunnelId())
                .tunnelCode(tunnel.getTunnelCode())
                .clusterId(tunnel.getClusterId())
                .description(tunnel.getDescription())
                .bandwidthUsed(tunnel.getBandwidthUsed())
                .expirationHours(tunnel.getExpirationHours())
                .tunnelExpiration(tunnelExpiration(tunnel))
                .created(tunnel.getCreatedAt())
                .url(tunnel.getUrl())
                .type(typeValue(tunnel))
                .build();
    }

    public static TunnelDetailResponse toDetailResponse(
            Tunnel tunnel, TunnelRuntimeStatus runtimeStatus) {
        TunnelDetailResponse.TunnelDetailResponseBuilder builder = TunnelDetailResponse.builder()
                .name(tunnel.getName())
                .tunnelId(tunnel.getTunnelId())
                .tunnelCode(tunnel.getTunnelCode())
                .clusterId(tunnel.getClusterId())
                .description(tunnel.getDescription())
                .bandwidthUsed(tunnel.getBandwidthUsed())
                .expirationHours(tunnel.getExpirationHours())
                .tunnelExpiration(tunnelExpiration(tunnel))
                .created(tunnel.getCreatedAt())
                .url(tunnel.getUrl())
                .type(typeValue(tunnel));
        if (runtimeStatus != null) {
            builder.status(TunnelStatusResponse.builder()
                    .hostConnectionCount(runtimeStatus.getHostConnectionCount())
                    .clientConnectionCount(runtimeStatus.getClientConnectionCount())
                    .uploadBytesPerSecond(runtimeStatus.getUploadBytesPerSecond())
                    .downloadBytesPerSecond(runtimeStatus.getDownloadBytesPerSecond())
                    .totalUploadBytes(runtimeStatus.getTotalUploadBytes())
                    .totalDownloadBytes(runtimeStatus.getTotalDownloadBytes())
                    .reportedAt(runtimeStatus.getReportedAt())
                    .build());
        }
        return builder.build();
    }

    public static TunnelListItemResponse toListItem(Tunnel tunnel) {
        return TunnelListItemResponse.builder()
                .tunnelId(tunnel.getTunnelId())
                .tunnelCode(tunnel.getTunnelCode())
                .clusterId(tunnel.getClusterId())
                .name(tunnel.getName())
                .description(tunnel.getDescription())
                .expirationHours(tunnel.getExpirationHours())
                .tunnelExpiration(tunnelExpiration(tunnel))
                .created(tunnel.getCreatedAt())
                .url(tunnel.getUrl())
                .portCount(tunnel.getPortCount() == null ? 0L : tunnel.getPortCount())
                .build();
    }

    private static String typeValue(Tunnel tunnel) {
        TunnelType type = tunnel.getType();
        return type == null ? null : type.value();
    }

    private static Long tunnelExpiration(Tunnel tunnel) {
        Integer expiration = tunnel.getExpiration();
        return expiration == null ? null : expiration.longValue();
    }
}
