package com.huawei.devbridge.relaycontroller.domain.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class TunnelRuntimeStatus {
    private Integer hostConnectionCount;
    private Integer clientConnectionCount;
    private Long uploadBytesPerSecond;
    private Long downloadBytesPerSecond;
    private Long totalUploadBytes;
    private Long totalDownloadBytes;
    private Long reportedAt;
}
