package com.huawei.devbridge.relaycontroller.interfaces.response;

import com.huawei.devbridge.relaycontroller.domain.model.TunnelControlAction;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelControlReason;
import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class TunnelStatusDecisionResponse {
    private String tunnelId;
    private TunnelControlAction action;
    private TunnelControlReason reason;
    private Long remainingBytes;
    private DataPlaneLimitsResponse limits;
}
