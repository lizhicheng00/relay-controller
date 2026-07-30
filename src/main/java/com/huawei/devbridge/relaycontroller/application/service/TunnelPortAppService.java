package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.application.assembler.TunnelPortAssembler;
import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.AccountPlan;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelPort;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelProtocol;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelPortRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.domain.service.NamespaceService;
import com.huawei.devbridge.relaycontroller.domain.service.TunnelDomainService;
import com.huawei.devbridge.relaycontroller.domain.service.TunnelPortDomainService;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import com.huawei.devbridge.relaycontroller.interfaces.request.CreateTunnelPortRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.UpdateTunnelPortRequest;
import com.huawei.devbridge.relaycontroller.interfaces.response.GatewayTunnelPortPolicyResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelPortResponse;
import java.util.List;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
@RequiredArgsConstructor
@Slf4j
public class TunnelPortAppService {
    private final TunnelRepository tunnelRepository;
    private final TunnelPortRepository tunnelPortRepository;
    private final LocalClusterService localClusterService;
    private final NamespaceService namespaceService;
    private final TunnelDomainService tunnelDomainService;
    private final TunnelPortDomainService tunnelPortDomainService;
    private final RelayProperties relayProperties;
    private final BillingService billingService;

    @Transactional
    public TunnelPortResponse create(String rawNamespace, String tunnelId, CreateTunnelPortRequest request) {
        Tunnel tunnel = ownedTunnelForUpdate(rawNamespace, tunnelId);
        tunnelPortDomainService.validatePort(request.getPort());
        tunnelPortDomainService.validateProtocol(request.getProtocol());
        tunnelPortDomainService.validateAllowAnonymous(request.getAllowAnonymous());
        if (tunnelPortRepository.existsByTunnelCodeAndPort(tunnel.getTunnelCode(), request.getPort())) {
            throw new BizException(ErrorCode.TUNNEL_PORT_ALREADY_EXISTS);
        }
        AccountPlan accountPlan = billingService.accountPlan(tunnel.getAccountId());
        int maxPorts = accountPlan.plan().getMaxPortsPerTunnel();
        long portCount = tunnelPortRepository.countByTunnelCode(tunnel.getTunnelCode());
        if (maxPorts > 0 && portCount >= maxPorts) {
            throw new BizException(ErrorCode.TUNNEL_PORT_QUOTA_EXCEEDED,
                    "tunnel port quota exceeded: max=" + maxPorts);
        }

        TunnelPort tunnelPort = tunnelPortRepository.save(TunnelPort.builder()
                .tunnelCode(tunnel.getTunnelCode())
                .port(request.getPort())
                .protocol(request.getProtocol())
                .allowAnonymous(request.getAllowAnonymous())
                .build());
        refreshExpiration(tunnel);
        log.info("Tunnel port created: tunnelId={}, tunnelCode={}, port={}, protocol={}, allowAnonymous={}",
                tunnel.getTunnelId(), tunnel.getTunnelCode(), tunnelPort.getPort(), tunnelPort.getProtocol(),
                tunnelPort.getAllowAnonymous());
        return TunnelPortAssembler.toResponse(tunnel, tunnelPort);
    }

    public List<TunnelPortResponse> list(String rawNamespace, String tunnelId) {
        Tunnel tunnel = ownedTunnel(rawNamespace, tunnelId);
        return tunnelPortRepository.findByTunnelCode(tunnel.getTunnelCode()).stream()
                .map(tunnelPort -> TunnelPortAssembler.toResponse(tunnel, tunnelPort))
                .toList();
    }

    public TunnelPortResponse detail(String rawNamespace, String tunnelId, Long port) {
        Tunnel tunnel = ownedTunnel(rawNamespace, tunnelId);
        TunnelPort tunnelPort = findTunnelPort(tunnel.getTunnelCode(), port);
        return TunnelPortAssembler.toResponse(tunnel, tunnelPort);
    }

    @Transactional
    public TunnelPortResponse update(String rawNamespace, String tunnelId, Long port, UpdateTunnelPortRequest request) {
        Tunnel tunnel = ownedTunnelForUpdate(rawNamespace, tunnelId);
        TunnelPort tunnelPort = findTunnelPort(tunnel.getTunnelCode(), port);
        TunnelProtocol protocol = request.getProtocol() == null ? tunnelPort.getProtocol() : request.getProtocol();
        Boolean allowAnonymous = request.getAllowAnonymous() == null
                ? tunnelPort.getAllowAnonymous()
                : request.getAllowAnonymous();
        if (request.getProtocol() != null || request.getAllowAnonymous() != null) {
            tunnelPortRepository.updatePolicy(tunnel.getTunnelCode(), port, protocol, allowAnonymous);
            refreshExpiration(tunnel);
            log.info("Tunnel port updated: tunnelId={}, tunnelCode={}, port={}, protocol={}, allowAnonymous={}",
                    tunnel.getTunnelId(), tunnel.getTunnelCode(), port, protocol, allowAnonymous);
        }
        tunnelPort.setProtocol(protocol);
        tunnelPort.setAllowAnonymous(allowAnonymous);
        return TunnelPortAssembler.toResponse(tunnel, tunnelPort);
    }

    @Transactional
    public Boolean delete(String rawNamespace, String tunnelId, Long port) {
        Tunnel tunnel = ownedTunnelForUpdate(rawNamespace, tunnelId);
        findTunnelPort(tunnel.getTunnelCode(), port);
        tunnelPortRepository.deleteByTunnelCodeAndPort(tunnel.getTunnelCode(), port);
        refreshExpiration(tunnel);
        log.info("Tunnel port deleted: tunnelId={}, tunnelCode={}, port={}",
                tunnel.getTunnelId(), tunnel.getTunnelCode(), port);
        return true;
    }

    public GatewayTunnelPortPolicyResponse getGatewayPortPolicy(String clusterId, String tunnelId, Long port) {
        localClusterService.requireLocalCluster(clusterId);
        Tunnel tunnel = tunnelRepository.findByTunnelIdAndRegion(tunnelId, relayProperties.getRegion());
        tunnelDomainService.assertInClusterAndNotExpired(tunnel, clusterId, ErrorCode.TUNNEL_PORT_ACCESS_DENIED);
        TunnelPort tunnelPort = findTunnelPort(tunnel.getTunnelCode(), port);
        return TunnelPortAssembler.toGatewayPolicy(tunnel, tunnelPort);
    }

    private Tunnel ownedTunnel(String rawNamespace, String tunnelId) {
        String namespace = namespaceService.requireNamespace(rawNamespace);
        Tunnel tunnel = tunnelRepository.findByTunnelIdAndRegion(tunnelId, relayProperties.getRegion());
        tunnelDomainService.assertOwnedAndNotExpired(tunnel, namespace);
        return tunnel;
    }

    private Tunnel ownedTunnelForUpdate(String rawNamespace, String tunnelId) {
        String namespace = namespaceService.requireNamespace(rawNamespace);
        List<String> clusterIds = localClusterService.localClusterIds();
        Tunnel tunnel = clusterIds.isEmpty()
                ? null
                : tunnelRepository.findByTunnelIdAndClustersForUpdate(tunnelId, clusterIds);
        tunnelDomainService.assertOwnedAndNotExpired(tunnel, namespace);
        return tunnel;
    }

    private TunnelPort findTunnelPort(Long tunnelCode, Long port) {
        tunnelPortDomainService.validatePort(port);
        TunnelPort tunnelPort = tunnelPortRepository.findByTunnelCodeAndPort(tunnelCode, port);
        if (tunnelPort == null) {
            throw new BizException(ErrorCode.TUNNEL_PORT_NOT_FOUND);
        }
        return tunnelPort;
    }

    private void refreshExpiration(Tunnel tunnel) {
        tunnelRepository.refreshExpiration(tunnel.getTunnelId(), relayProperties.getRegion(), TimeUtils.nowSeconds());
    }
}
