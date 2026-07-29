package com.huawei.devbridge.relaycontroller.infrastructure.persistence.converter;

import com.huawei.devbridge.relaycontroller.domain.model.Cluster;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelPort;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.entity.ClusterEntity;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.entity.TunnelEntity;
import com.huawei.devbridge.relaycontroller.infrastructure.persistence.entity.TunnelPortEntity;
import org.mapstruct.Mapper;

@Mapper(componentModel = "spring")
public interface PersistenceConverter {
    Cluster toDomain(ClusterEntity entity);

    Tunnel toDomain(TunnelEntity entity);

    TunnelEntity toEntity(Tunnel tunnel);

    TunnelPort toDomain(TunnelPortEntity entity);

    TunnelPortEntity toEntity(TunnelPort tunnelPort);
}
