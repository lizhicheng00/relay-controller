package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.repository.MeteringPartitionRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class MeteringPartitionJob {
    private final MeteringPartitionRepository partitionRepository;

    @Scheduled(initialDelay = 60000, fixedDelay = 3600000)
    public void maintainPartitions() {
        maintainPartitions(TimeUtils.nowSeconds());
    }

    void maintainPartitions(long now) {
        partitionRepository.maintain(now);
    }
}
