package com.huawei.devbridge.relaycontroller.infrastructure.persistence.mapper;

import com.huawei.devbridge.relaycontroller.domain.model.TunnelRuntimeStatus;
import java.util.List;
import org.apache.ibatis.annotations.Param;

public interface TunnelRuntimeStatusMapper {
    int upsertAll(@Param("statuses") List<TunnelRuntimeStatus> statuses);

    int deleteStale(@Param("reportedBefore") long reportedBefore);
}
