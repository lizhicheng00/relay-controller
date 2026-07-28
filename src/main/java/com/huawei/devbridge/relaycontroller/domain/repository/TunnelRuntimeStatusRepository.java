package com.huawei.devbridge.relaycontroller.domain.repository;

import com.huawei.devbridge.relaycontroller.domain.model.TunnelRuntimeStatus;
import java.util.List;

public interface TunnelRuntimeStatusRepository {
    void upsertAll(List<TunnelRuntimeStatus> statuses);

    int deleteStale(long reportedBefore);
}
