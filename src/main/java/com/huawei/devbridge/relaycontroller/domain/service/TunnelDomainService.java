package com.huawei.devbridge.relaycontroller.domain.service;

import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import org.springframework.stereotype.Service;

@Service
public class TunnelDomainService {

    public void assertActive(Tunnel tunnel) {
        if (tunnel == null || Integer.valueOf(1).equals(tunnel.getDeleted())) {
            throw new BizException(ErrorCode.TUNNEL_NOT_FOUND);
        }
    }

    public void assertOwnedBy(Tunnel tunnel, String namespace) {
        assertActive(tunnel);
        if (!namespace.equals(tunnel.getNamespace())) {
            throw new BizException(ErrorCode.TUNNEL_ACCESS_DENIED);
        }
    }

    public void assertOwnedAndNotExpired(Tunnel tunnel, String namespace) {
        assertOwnedBy(tunnel, namespace);
        assertNotExpired(tunnel);
    }

    public void assertNotExpired(Tunnel tunnel) {
        assertActive(tunnel);
        if (tunnel.getExpiration() != null && tunnel.getExpiration() <= TimeUtils.nowSeconds()) {
            throw new BizException(ErrorCode.TUNNEL_EXPIRED);
        }
    }
}
