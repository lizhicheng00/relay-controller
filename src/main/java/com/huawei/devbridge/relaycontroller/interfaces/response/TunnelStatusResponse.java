package com.huawei.devbridge.relaycontroller.interfaces.response;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class TunnelStatusResponse {
    private Integer hostConnectionCount;
    private Integer clientConnectionCount;
    private Long uploadBytesPerSecond;
    private Long downloadBytesPerSecond;
    private Long totalUploadBytes;
    private Long totalDownloadBytes;
    private Long reportedAt;
}
