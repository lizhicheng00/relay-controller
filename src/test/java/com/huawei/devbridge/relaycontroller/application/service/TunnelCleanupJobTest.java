package com.huawei.devbridge.relaycontroller.application.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelPortRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRuntimeStatusRepository;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
class TunnelCleanupJobTest {
    private static final long NOW = 400000L;
    private static final long EXPIRATION_CUTOFF = NOW - 3 * 86400L;

    @Mock
    private TunnelRepository tunnelRepository;
    @Mock
    private TunnelPortRepository tunnelPortRepository;
    @Mock
    private TunnelRuntimeStatusRepository statusRepository;

    @Test
    void shouldHardDeleteAgedTunnelsAndCleanRelatedState() {
        RelayProperties properties = new RelayProperties();
        TunnelCleanupJob cleanupJob = new TunnelCleanupJob(
                tunnelRepository, tunnelPortRepository, statusRepository, properties);
        Tunnel tunnel = Tunnel.builder()
                .tunnelId("aaaadysa")
                .tunnelCode(123456L)
                .build();

        when(tunnelRepository.findAgedByRegion("region-a", EXPIRATION_CUTOFF, 100)).thenReturn(List.of(tunnel));
        when(tunnelRepository.deleteAgedByTunnelId("aaaadysa", EXPIRATION_CUTOFF)).thenReturn(true);

        int deleted = cleanupJob.cleanupAgedTunnels(NOW);

        assertThat(deleted).isEqualTo(1);
        verify(tunnelRepository).deleteAgedByTunnelId("aaaadysa", EXPIRATION_CUTOFF);
        verify(tunnelPortRepository).deleteByTunnelCode(123456L);
        verify(statusRepository).deleteStale(NOW - 86400L);
    }

    @Test
    void shouldSkipRelatedCleanupWhenTunnelWasRenewed() {
        RelayProperties properties = new RelayProperties();
        TunnelCleanupJob cleanupJob = new TunnelCleanupJob(
                tunnelRepository, tunnelPortRepository, statusRepository, properties);
        Tunnel tunnel = Tunnel.builder()
                .tunnelId("aaaadysa")
                .tunnelCode(123456L)
                .build();

        when(tunnelRepository.findAgedByRegion("region-a", EXPIRATION_CUTOFF, 100)).thenReturn(List.of(tunnel));
        when(tunnelRepository.deleteAgedByTunnelId("aaaadysa", EXPIRATION_CUTOFF)).thenReturn(false);

        int deleted = cleanupJob.cleanupAgedTunnels(NOW);

        assertThat(deleted).isZero();
        verify(tunnelPortRepository, never()).deleteByTunnelCode(123456L);
    }
}
