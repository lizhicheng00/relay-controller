package com.huawei.devbridge.relaycontroller.infrastructure.persistence.repository;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.converter.PersistenceConverter;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.entity.TunnelEntity;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.mapper.TunnelMapper;
import java.util.List;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;

@Repository
@RequiredArgsConstructor
public class TunnelRepositoryImpl implements TunnelRepository {
    private final TunnelMapper tunnelMapper;
    private final PersistenceConverter converter;

    @Override
    public Tunnel findByTunnelIdAndRegion(String tunnelId, String region) {
        return converter.toDomain(tunnelMapper.selectByTunnelIdAndRegion(tunnelId, region));
    }

    @Override
    public Tunnel findByTunnelIdAndClustersForUpdate(String tunnelId, List<String> clusterIds) {
        return converter.toDomain(tunnelMapper.selectByTunnelIdAndClustersForUpdate(tunnelId, clusterIds));
    }

    @Override
    public List<Tunnel> findByNamespaceAndRegion(String namespace, String region) {
        return tunnelMapper.selectByNamespaceAndRegion(namespace, region).stream()
                .map(converter::toDomain)
                .toList();
    }

    @Override
    public List<Tunnel> findActiveByNamespaceAndRegion(String namespace, String clusterId, String region, long now) {
        return tunnelMapper.selectActiveByNamespaceAndRegion(namespace, clusterId, region, now).stream()
                .map(converter::toDomain)
                .toList();
    }

    @Override
    public List<Tunnel> findAgedByRegion(String region, long expirationCutoff, int limit) {
        return tunnelMapper.selectAgedByRegion(region, expirationCutoff, limit).stream()
                .map(converter::toDomain)
                .toList();
    }

    @Override
    public long countActiveByAccountId(Long accountId, long now) {
        return tunnelMapper.countActiveByAccountId(accountId, now);
    }

    @Override
    public boolean existsByTunnelId(String tunnelId) {
        return tunnelMapper.exists(new LambdaQueryWrapper<TunnelEntity>()
                .eq(TunnelEntity::getTunnelId, tunnelId));
    }

    @Override
    public boolean existsByTunnelCode(Long tunnelCode) {
        return tunnelMapper.exists(new LambdaQueryWrapper<TunnelEntity>()
                .eq(TunnelEntity::getTunnelCode, tunnelCode));
    }

    @Override
    public Tunnel save(Tunnel tunnel) {
        TunnelEntity entity = converter.toEntity(tunnel);
        tunnelMapper.insert(entity);
        tunnel.setId(entity.getId());
        return tunnel;
    }

    @Override
    public void update(Tunnel tunnel) {
        tunnelMapper.updateById(converter.toEntity(tunnel));
    }

    @Override
    public void refreshExpiration(String tunnelId, String region, long activityAt) {
        tunnelMapper.refreshExpiration(tunnelId, region, activityAt);
    }

    @Override
    public boolean deleteAgedByTunnelId(String tunnelId, long expirationCutoff) {
        return tunnelMapper.delete(new LambdaQueryWrapper<TunnelEntity>()
                .eq(TunnelEntity::getTunnelId, tunnelId)
                .le(TunnelEntity::getExpiration, expirationCutoff)) > 0;
    }

    @Override
    public boolean deleteByTunnelId(String tunnelId) {
        return tunnelMapper.delete(new LambdaQueryWrapper<TunnelEntity>()
                .eq(TunnelEntity::getTunnelId, tunnelId)) > 0;
    }

    @Override
    public void increaseBandwidthUsed(String tunnelId, String region, long usageBytes, long updatedAt) {
        tunnelMapper.increaseBandwidthUsed(tunnelId, region, usageBytes, updatedAt);
    }

}
