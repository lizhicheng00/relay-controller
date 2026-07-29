package com.huawei.devbridge.relaycontroller.infrastructure.persistence.repository;

import com.huawei.devbridge.relaycontroller.domain.model.TunnelRuntimeStatus;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRuntimeStatusRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.mapper.TunnelRuntimeStatusMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;

@Repository
@RequiredArgsConstructor
public class TunnelRuntimeStatusRepositoryImpl implements TunnelRuntimeStatusRepository {
    private final TunnelRuntimeStatusMapper mapper;

    @Override
    public TunnelRuntimeStatus findByTunnelId(String tunnelId) {
        return mapper.selectByTunnelId(tunnelId);
    }

    @Override
    public int deleteStale(long reportedBefore) {
        return mapper.deleteStale(reportedBefore);
    }
}
