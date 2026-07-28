package com.huawei.devbridge.relaycontroller.interfaces.request;

import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import jakarta.validation.constraints.Size;
import lombok.Data;

@Data
public class TunnelStatusItemRequest {
    @NotBlank
    @Pattern(regexp = "^[a-z2-7]{8}$")
    private String tunnelId;
    @Size(max = 128)
    private String sessionId;
    @NotNull
    @Min(0)
    private Integer hostConnections;
    @NotNull
    @Min(0)
    private Integer clientConnections;
    @NotNull
    @Min(0)
    private Integer channelCount;
    @NotNull
    @Min(0)
    private Long uploadBytesPerSecond;
    @NotNull
    @Min(0)
    private Long downloadBytesPerSecond;
}
