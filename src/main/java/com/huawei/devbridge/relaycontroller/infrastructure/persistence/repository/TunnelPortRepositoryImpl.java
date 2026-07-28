package com.huawei.devbridge.relaycontroller.infrastructure.persistence.repository;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.LambdaUpdateWrapper;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelPort;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelProtocol;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelPortRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.converter.PersistenceConverter;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.entity.TunnelPortEntity;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.mapper.TunnelPortMapper;
import java.util.List;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;

@Repository
@RequiredArgsConstructor
public class TunnelPortRepositoryImpl implements TunnelPortRepository {
    private final TunnelPortMapper tunnelPortMapper;
    private final PersistenceConverter converter;

    @Override
    public TunnelPort save(TunnelPort tunnelPort) {
        TunnelPortEntity entity = converter.toEntity(tunnelPort);
        tunnelPortMapper.insert(entity);
        tunnelPort.setId(entity.getId());
        return tunnelPort;
    }

    @Override
    public List<TunnelPort> findByTunnelCode(Long tunnelCode) {
        return tunnelPortMapper.selectList(new LambdaQueryWrapper<TunnelPortEntity>()
                        .eq(TunnelPortEntity::getTunnelCode, tunnelCode)
                        .orderByAsc(TunnelPortEntity::getPort))
                .stream()
                .map(converter::toDomain)
                .toList();
    }

    @Override
    public TunnelPort findByTunnelCodeAndPort(Long tunnelCode, Long port) {
        TunnelPortEntity entity = tunnelPortMapper.selectOne(new LambdaQueryWrapper<TunnelPortEntity>()
                .eq(TunnelPortEntity::getTunnelCode, tunnelCode)
                .eq(TunnelPortEntity::getPort, port)
                .last("LIMIT 1"));
        return converter.toDomain(entity);
    }

    @Override
    public boolean existsByTunnelCodeAndPort(Long tunnelCode, Long port) {
        return tunnelPortMapper.exists(new LambdaQueryWrapper<TunnelPortEntity>()
                .eq(TunnelPortEntity::getTunnelCode, tunnelCode)
                .eq(TunnelPortEntity::getPort, port));
    }

    @Override
    public long countByTunnelCode(Long tunnelCode) {
        return tunnelPortMapper.selectCount(new LambdaQueryWrapper<TunnelPortEntity>()
                .eq(TunnelPortEntity::getTunnelCode, tunnelCode));
    }

    @Override
    public void updatePolicy(Long tunnelCode, Long port, TunnelProtocol protocol, Boolean allowAnonymous) {
        tunnelPortMapper.update(null, new LambdaUpdateWrapper<TunnelPortEntity>()
                .eq(TunnelPortEntity::getTunnelCode, tunnelCode)
                .eq(TunnelPortEntity::getPort, port)
                .set(TunnelPortEntity::getProtocol, protocol)
                .set(TunnelPortEntity::getAllowAnonymous, allowAnonymous));
    }

    @Override
    public void deleteByTunnelCodeAndPort(Long tunnelCode, Long port) {
        tunnelPortMapper.delete(new LambdaQueryWrapper<TunnelPortEntity>()
                .eq(TunnelPortEntity::getTunnelCode, tunnelCode)
                .eq(TunnelPortEntity::getPort, port));
    }

    @Override
    public void deleteByTunnelCode(Long tunnelCode) {
        tunnelPortMapper.delete(new LambdaQueryWrapper<TunnelPortEntity>()
                .eq(TunnelPortEntity::getTunnelCode, tunnelCode));
    }

}
