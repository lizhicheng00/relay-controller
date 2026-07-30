package com.huawei.devbridge.relaycontroller.domain.repository;

import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import java.util.List;

public interface TunnelRepository {
    Tunnel findByTunnelIdAndRegion(String tunnelId, String region);

    Tunnel findByTunnelIdAndClustersForUpdate(String tunnelId, List<String> clusterIds);

    List<Tunnel> findByNamespaceAndRegion(String namespace, String region);

    List<Tunnel> findActiveByNamespaceAndRegion(String namespace, String clusterId, String region, long now);

    List<Tunnel> findAgedByRegion(String region, long expirationCutoff, int limit);

    long countActiveByAccountId(Long accountId, long now);

    boolean existsByTunnelId(String tunnelId);

    boolean existsByTunnelCode(Long tunnelCode);

    Tunnel save(Tunnel tunnel);

    void update(Tunnel tunnel);

    void refreshExpiration(String tunnelId, String region, long activityAt);

    boolean deleteAgedByTunnelId(String tunnelId, long expirationCutoff);

    boolean deleteByTunnelId(String tunnelId);

    void increaseBandwidthUsed(String tunnelId, String region, long usageBytes, long updatedAt);
}
