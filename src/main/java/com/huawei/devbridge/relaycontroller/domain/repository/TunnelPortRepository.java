package com.huawei.devbridge.relaycontroller.domain.repository;

import com.huawei.devbridge.relaycontroller.domain.model.TunnelPort;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelProtocol;
import java.util.List;

public interface TunnelPortRepository {
    TunnelPort save(TunnelPort tunnelPort);

    List<TunnelPort> findByTunnelCode(Long tunnelCode);

    TunnelPort findByTunnelCodeAndPort(Long tunnelCode, Long port);

    boolean existsByTunnelCodeAndPort(Long tunnelCode, Long port);

    long countByTunnelCode(Long tunnelCode);

    void updatePolicy(Long tunnelCode, Long port, TunnelProtocol protocol, Boolean allowAnonymous);

    void deleteByTunnelCodeAndPort(Long tunnelCode, Long port);

    void deleteByTunnelCode(Long tunnelCode);
}
