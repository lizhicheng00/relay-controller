package com.huawei.devbridge.relaycontroller.domain.repository;

import com.huawei.devbridge.relaycontroller.domain.model.Cluster;
import java.util.List;

public interface ClusterRepository {
    Cluster findByClusterIdAndRegion(String clusterId, String region);

    List<String> findIdsByRegion(String region);
}
