package com.huawei.devbridge.relaycontroller.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.entity.TunnelEntity;
import java.util.List;
import org.apache.ibatis.annotations.Param;

public interface TunnelMapper extends BaseMapper<TunnelEntity> {
    TunnelEntity selectByTunnelIdAndRegion(
            @Param("tunnelId") String tunnelId,
            @Param("region") String region);

    TunnelEntity selectByTunnelIdAndRegionForUpdate(
            @Param("tunnelId") String tunnelId,
            @Param("region") String region);

    List<TunnelEntity> selectByTunnelIdsAndRegion(
            @Param("tunnelIds") List<String> tunnelIds,
            @Param("region") String region);

    List<TunnelEntity> selectByNamespaceAndRegion(
            @Param("namespace") String namespace,
            @Param("region") String region);

    List<TunnelEntity> selectActiveByNamespaceAndRegion(
            @Param("namespace") String namespace,
            @Param("clusterId") String clusterId,
            @Param("region") String region,
            @Param("now") long now);

    List<TunnelEntity> selectAgedByRegion(
            @Param("region") String region,
            @Param("expirationCutoff") long expirationCutoff,
            @Param("limit") int limit);

    long countActiveByNamespaceAndRegion(
            @Param("namespace") String namespace,
            @Param("region") String region,
            @Param("now") long now);

    long countActiveByAccountId(
            @Param("accountId") Long accountId,
            @Param("now") long now);

    int increaseBandwidthUsed(
            @Param("tunnelId") String tunnelId,
            @Param("region") String region,
            @Param("usageBytes") long usageBytes,
            @Param("updatedAt") long updatedAt);

    int refreshExpiration(
            @Param("tunnelId") String tunnelId,
            @Param("region") String region,
            @Param("activityAt") long activityAt);

    int refreshExpirationFromHeartbeat(
            @Param("tunnelId") String tunnelId,
            @Param("region") String region,
            @Param("activityAt") long activityAt,
            @Param("minimumExtensionSeconds") int minimumExtensionSeconds);
}
