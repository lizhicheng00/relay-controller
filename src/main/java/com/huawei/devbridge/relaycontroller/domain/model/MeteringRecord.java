package com.huawei.devbridge.relaycontroller.domain.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class MeteringRecord {
    private Long id;
    private Long accountId;
    private String tunnelId;
    private Long uploadBytes;
    private Long downloadBytes;
    private Long reportedAt;

    public long totalBytes() {
        return Math.addExact(uploadBytes, downloadBytes);
    }
}
