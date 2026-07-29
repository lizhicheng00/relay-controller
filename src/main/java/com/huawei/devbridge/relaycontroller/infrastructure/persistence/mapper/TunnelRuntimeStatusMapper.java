package com.huawei.devbridge.relaycontroller.infrastructure.persistence.mapper;

import com.huawei.devbridge.relaycontroller.domain.model.TunnelRuntimeStatus;
import org.apache.ibatis.annotations.Param;

public interface TunnelRuntimeStatusMapper {
    TunnelRuntimeStatus selectByTunnelId(@Param("tunnelId") String tunnelId);

    int deleteStale(@Param("reportedBefore") long reportedBefore);
}
