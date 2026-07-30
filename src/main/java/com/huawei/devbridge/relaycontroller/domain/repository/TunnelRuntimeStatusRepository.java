package com.huawei.devbridge.relaycontroller.domain.repository;

import com.huawei.devbridge.relaycontroller.domain.model.TunnelRuntimeStatus;

public interface TunnelRuntimeStatusRepository {
    TunnelRuntimeStatus findByTunnelId(String tunnelId);

    void deleteByTunnelId(String tunnelId);
}
