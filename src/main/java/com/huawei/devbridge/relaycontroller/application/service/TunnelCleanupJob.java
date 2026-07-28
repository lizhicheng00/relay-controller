package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelPortRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRuntimeStatusRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import java.util.List;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
@RequiredArgsConstructor
@Slf4j
public class TunnelCleanupJob {
    private static final int BATCH_SIZE = 100;
    private static final long SECONDS_PER_DAY = 86400L;

    private final TunnelRepository tunnelRepository;
    private final TunnelPortRepository tunnelPortRepository;
    private final TunnelRuntimeStatusRepository statusRepository;
    private final RelayProperties relayProperties;

    @Scheduled(
            initialDelayString = "${relay.tunnel.cleanup-initial-delay-ms:60000}",
            fixedDelayString = "${relay.tunnel.cleanup-interval-ms:600000}")
    @Transactional
    public void cleanupAgedTunnels() {
        cleanupAgedTunnels(TimeUtils.nowSeconds());
    }

    int cleanupAgedTunnels(long now) {
        long expirationCutoff = expirationCutoff(now);
        List<Tunnel> agedTunnels = tunnelRepository.findAgedByRegion(relayProperties.getRegion(), expirationCutoff, BATCH_SIZE);
        int deleted = 0;
        for (Tunnel tunnel : agedTunnels) {
            if (tunnelRepository.deleteAgedByTunnelId(tunnel.getTunnelId(), expirationCutoff)) {
                tunnelPortRepository.deleteByTunnelCode(tunnel.getTunnelCode());
                deleted++;
            }
        }
        long statusRetention = Math.max(0, relayProperties.getBilling().getStatusRetentionSeconds());
        statusRepository.deleteStale(now - statusRetention);
        if (deleted > 0) {
            log.info("Aged tunnels deleted: region={}, count={}", relayProperties.getRegion(), deleted);
        }
        return deleted;
    }

    private long expirationCutoff(long now) {
        int retentionDays = Math.max(0, relayProperties.getTunnel().getCleanupRetentionDays());
        return now - retentionDays * SECONDS_PER_DAY;
    }
}
