package com.huawei.devbridge.relaycontroller.infrastructure.persistence.repository;

import com.huawei.devbridge.relaycontroller.domain.model.TunnelRuntimeStatus;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRuntimeStatusRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.mapper.TunnelRuntimeStatusMapper;
import java.util.List;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;

@Repository
@RequiredArgsConstructor
public class TunnelRuntimeStatusRepositoryImpl implements TunnelRuntimeStatusRepository {
    private final TunnelRuntimeStatusMapper mapper;

    @Override
    public void upsertAll(List<TunnelRuntimeStatus> statuses) {
        if (!statuses.isEmpty()) {
            mapper.upsertAll(statuses);
        }
    }

    @Override
    public int deleteStale(long reportedBefore) {
        return mapper.deleteStale(reportedBefore);
    }
}
