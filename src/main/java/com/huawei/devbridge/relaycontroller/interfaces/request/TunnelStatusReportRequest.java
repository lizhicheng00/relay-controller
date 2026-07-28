package com.huawei.devbridge.relaycontroller.interfaces.request;

import com.huawei.devbridge.relaycontroller.common.validation.IdentifierValidator;
import jakarta.validation.Valid;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import jakarta.validation.constraints.Size;
import java.util.List;
import lombok.Data;

@Data
public class TunnelStatusReportRequest {
    @NotBlank
    @Pattern(regexp = IdentifierValidator.REGEX)
    private String gatewayId;
    @NotNull
    @Min(0)
    private Long reportedAt;
    @NotNull
    @Size(max = 500)
    private List<@NotNull @Valid TunnelStatusItemRequest> tunnels;
}
