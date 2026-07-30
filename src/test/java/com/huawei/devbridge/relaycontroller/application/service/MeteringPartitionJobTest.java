package com.huawei.devbridge.relaycontroller.application.service;

import static org.mockito.Mockito.verify;

import com.huawei.devbridge.relaycontroller.domain.repository.MeteringPartitionRepository;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
class MeteringPartitionJobTest {
    @Mock
    private MeteringPartitionRepository partitionRepository;

    @Test
    void delegatesPartitionMaintenance() {
        MeteringPartitionJob job = new MeteringPartitionJob(partitionRepository);

        job.maintainPartitions(1785373200L);

        verify(partitionRepository).maintain(1785373200L);
    }
}
