package com.huawei.devbridge.relaycontroller.application.service;

import com.huawei.devbridge.relaycontroller.application.assembler.TunnelAssembler;
import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import com.huawei.devbridge.relaycontroller.domain.model.AccountPlan;
import com.huawei.devbridge.relaycontroller.domain.model.Cluster;
import com.huawei.devbridge.relaycontroller.domain.model.JwtScope;
import com.huawei.devbridge.relaycontroller.domain.model.JwtToken;
import com.huawei.devbridge.relaycontroller.domain.model.Tunnel;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelType;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelPortRepository;
import com.huawei.devbridge.relaycontroller.domain.repository.TunnelRepository;
import com.huawei.devbridge.relaycontroller.domain.service.JwtTokenService;
import com.huawei.devbridge.relaycontroller.domain.service.NamespaceService;
import com.huawei.devbridge.relaycontroller.domain.service.TunnelCodeGenerator;
import com.huawei.devbridge.relaycontroller.domain.service.TunnelDomainService;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import com.huawei.devbridge.relaycontroller.interfaces.request.CreateTunnelRequest;
import com.huawei.devbridge.relaycontroller.interfaces.request.UpdateTunnelRequest;
import com.huawei.devbridge.relaycontroller.interfaces.response.CreateTunnelResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelDetailResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelListItemResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelTokenResponse;
import java.util.List;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
@RequiredArgsConstructor
@Slf4j
public class TunnelAppService {
    private static final int TUNNEL_CODE_MAX_RETRY = 5;
    private static final int SECONDS_PER_HOUR = 3600;
    private static final int MAX_EXPIRATION_HOURS = 30 * 24;
    private final TunnelRepository tunnelRepository;
    private final LocalClusterService localClusterService;
    private final NamespaceService namespaceService;
    private final TunnelCodeGenerator tunnelCodeGenerator;
    private final JwtTokenService jwtTokenService;
    private final TunnelDomainService tunnelDomainService;
    private final TunnelPortRepository tunnelPortRepository;
    private final RelayProperties relayProperties;
    private final BillingService billingService;

    @Transactional
    public CreateTunnelResponse createTunnel(String rawNamespace, CreateTunnelRequest request) {
        String namespace = namespaceService.requireNamespace(rawNamespace);
        TunnelType type = request.getType() == null ? TunnelType.BRIDGE : request.getType();
        Cluster cluster = localClusterService.requireLocalCluster(request.getClusterId());
        long now = TimeUtils.nowSeconds();
        AccountPlan accountPlan = billingService.lockAccountForQuota(namespace);
        assertTunnelQuota(accountPlan, now);
        TunnelExpiration expiration = resolveExpiration(request.getExpiration(), now);
        TunnelCode code = allocateTunnelCode();
        Tunnel tunnel = Tunnel.builder()
                .name(request.getName())
                .tunnelId(code.tunnelId())
                .tunnelCode(code.tunnelCode())
                .clusterId(cluster.getClusterId())
                .expiration(expiration.expiresAt())
                .expirationHours(expiration.expirationHours())
                .namespace(namespace)
                .accountId(accountPlan.account().getId())
                .description(request.getDescription())
                .bandwidthUsed(0L)
                .url(buildTunnelUrl(code.tunnelId(), cluster))
                .type(type)
                .deleted(0)
                .createdAt(now)
                .updatedAt(now)
                .build();
        tunnelRepository.save(tunnel);
        log.info("Tunnel created: tunnelId={}, tunnelCode={}, namespace={}, clusterId={}, type={}, expiresAt={}",
                tunnel.getTunnelId(), tunnel.getTunnelCode(), tunnel.getNamespace(), tunnel.getClusterId(),
                tunnel.getType(), tunnel.getExpiration());
        return TunnelAssembler.toCreateResponse(tunnel);
    }

    public List<TunnelListItemResponse> listTunnels(String rawNamespace, String clusterId) {
        String namespace = namespaceService.requireNamespace(rawNamespace);
        long now = TimeUtils.nowSeconds();
        if (clusterId != null && !clusterId.isBlank()) {
            localClusterService.requireLocalCluster(clusterId);
            return tunnelRepository.findActiveByNamespaceAndRegion(namespace, clusterId, relayProperties.getRegion(), now).stream()
                    .map(TunnelAssembler::toListItem)
                    .toList();
        }
        return tunnelRepository.findActiveByNamespaceAndRegion(namespace, null, relayProperties.getRegion(), now).stream()
                .map(TunnelAssembler::toListItem)
                .toList();
    }

    public TunnelDetailResponse getTunnelDetail(String rawNamespace, String tunnelId) {
        Tunnel tunnel = findOwnedTunnel(rawNamespace, tunnelId);
        tunnelDomainService.assertNotExpired(tunnel);
        return TunnelAssembler.toDetailResponse(tunnel);
    }

    public TunnelTokenResponse issueToken(
            String rawNamespace, String tunnelId, String scopeValue, Boolean forCookies) {
        JwtScope scope = parseScope(scopeValue);
        Tunnel tunnel = findOwnedActiveTunnel(rawNamespace, tunnelId);
        billingService.assertTrafficAllowed(tunnel.getNamespace());
        JwtToken issuedToken = jwtTokenService.issueToken(tunnel, scope, Boolean.TRUE.equals(forCookies));
        return TunnelTokenResponse.builder()
                .tunnelId(tunnel.getTunnelId())
                .scope(scope)
                .lifetime(issuedToken.lifetime())
                .expiration(issuedToken.expiration())
                .token(issuedToken.token())
                .build();
    }

    @Transactional
    public Boolean updateTunnel(String rawNamespace, String tunnelId, UpdateTunnelRequest request) {
        Tunnel tunnel = findOwnedTunnelForUpdate(rawNamespace, tunnelId);
        tunnelDomainService.assertNotExpired(tunnel);
        long now = TimeUtils.nowSeconds();
        applyUpdates(tunnel, request, now);
        tunnel.setUpdatedAt(now);
        tunnelRepository.update(tunnel);
        log.info("Tunnel updated: tunnelId={}, namespace={}", tunnel.getTunnelId(), tunnel.getNamespace());
        return true;
    }

    @Transactional
    public Boolean deleteTunnel(String rawNamespace, String tunnelId) {
        Tunnel tunnel = findOwnedTunnelForUpdate(rawNamespace, tunnelId);
        tunnelPortRepository.deleteByTunnelCode(tunnel.getTunnelCode());
        tunnelRepository.deleteByTunnelId(tunnelId);
        log.info("Tunnel deleted: tunnelId={}, tunnelCode={}, namespace={}",
                tunnel.getTunnelId(), tunnel.getTunnelCode(), tunnel.getNamespace());
        return true;
    }

    @Transactional
    public Boolean deleteTunnels(String rawNamespace) {
        String namespace = namespaceService.requireNamespace(rawNamespace);
        List<Tunnel> tunnels = tunnelRepository.findByNamespaceAndRegion(namespace, relayProperties.getRegion());
        int deleted = 0;
        for (Tunnel tunnel : tunnels) {
            Tunnel locked = tunnelRepository.findByTunnelIdAndRegionForUpdate(
                    tunnel.getTunnelId(), relayProperties.getRegion());
            if (locked == null) {
                continue;
            }
            tunnelDomainService.assertOwnedBy(locked, namespace);
            tunnelPortRepository.deleteByTunnelCode(locked.getTunnelCode());
            tunnelRepository.deleteByTunnelId(locked.getTunnelId());
            deleted++;
        }
        log.info("Tunnels deleted: namespace={}, count={}", namespace, deleted);
        return true;
    }

    private void applyUpdates(Tunnel tunnel, UpdateTunnelRequest request, long now) {
        if (request.getName() != null && !request.getName().isBlank()) {
            tunnel.setName(request.getName());
        }
        if (request.getDescription() != null) {
            tunnel.setDescription(request.getDescription());
        }
        if (request.getExpiration() != null) {
            TunnelExpiration expiration = resolveExpiration(request.getExpiration(), now);
            tunnel.setExpirationHours(expiration.expirationHours());
        }
        tunnel.setExpiration(resolveExpiration(tunnel.getExpirationHours(), now).expiresAt());
        if (request.getType() != null) {
            tunnel.setType(request.getType());
        }
    }

    private TunnelCode allocateTunnelCode() {
        for (int i = 0; i < TUNNEL_CODE_MAX_RETRY; i++) {
            long tunnelCode = tunnelCodeGenerator.generate();
            String tunnelId = tunnelCodeGenerator.toTunnelId(tunnelCode);
            if (!tunnelRepository.existsByTunnelCode(tunnelCode) && !tunnelRepository.existsByTunnelId(tunnelId)) {
                return new TunnelCode(tunnelCode, tunnelId);
            }
        }
        throw new BizException(ErrorCode.TUNNEL_ID_CONFLICT);
    }

    private void assertTunnelQuota(AccountPlan accountPlan, long now) {
        int maxTunnels = accountPlan.plan().getMaxTunnels();
        if (maxTunnels <= 0) {
            return;
        }
        long activeCount = tunnelRepository.countActiveByAccountId(accountPlan.account().getId(), now);
        if (activeCount >= maxTunnels) {
            throw new BizException(ErrorCode.TUNNEL_QUOTA_EXCEEDED,
                    "active tunnel quota exceeded: max=" + maxTunnels);
        }
    }

    private Tunnel findOwnedTunnel(String rawNamespace, String tunnelId) {
        String namespace = namespaceService.requireNamespace(rawNamespace);
        Tunnel tunnel = tunnelRepository.findByTunnelIdAndRegion(tunnelId, relayProperties.getRegion());
        tunnelDomainService.assertOwnedBy(tunnel, namespace);
        return tunnel;
    }

    private Tunnel findOwnedActiveTunnel(String rawNamespace, String tunnelId) {
        String namespace = namespaceService.requireNamespace(rawNamespace);
        Tunnel tunnel = tunnelRepository.findByTunnelIdAndRegion(tunnelId, relayProperties.getRegion());
        tunnelDomainService.assertOwnedAndNotExpired(tunnel, namespace);
        return tunnel;
    }

    private Tunnel findOwnedTunnelForUpdate(String rawNamespace, String tunnelId) {
        String namespace = namespaceService.requireNamespace(rawNamespace);
        Tunnel tunnel = tunnelRepository.findByTunnelIdAndRegionForUpdate(tunnelId, relayProperties.getRegion());
        tunnelDomainService.assertOwnedBy(tunnel, namespace);
        return tunnel;
    }

    private TunnelExpiration resolveExpiration(Integer expirationHours, long now) {
        int hours = expirationHours == null ? relayProperties.getDefaultExpirationHours() : expirationHours;
        if (hours <= 0) {
            throw new BizException(ErrorCode.PARAM_INVALID, "expiration must be positive hours");
        }
        if (hours > MAX_EXPIRATION_HOURS) {
            throw new BizException(ErrorCode.PARAM_INVALID, "expiration must be less than or equal to 720 hours");
        }
        long expiresAt = now + (long) hours * SECONDS_PER_HOUR;
        if (expiresAt > Integer.MAX_VALUE) {
            throw new BizException(ErrorCode.PARAM_INVALID, "expiration is too large");
        }
        return new TunnelExpiration(hours, Math.toIntExact(expiresAt));
    }

    private JwtScope parseScope(String scopeValue) {
        try {
            JwtScope scope = JwtScope.fromValue(scopeValue);
            if (scope == null) {
                throw new IllegalArgumentException();
            }
            return scope;
        } catch (IllegalArgumentException exception) {
            throw new BizException(ErrorCode.PARAM_INVALID, "scope must be host or connect");
        }
    }

    private String buildTunnelUrl(String tunnelId, Cluster cluster) {
        return tunnelId + "." + cluster.getClusterId() + "." + relayProperties.getDomain();
    }

    private record TunnelCode(long tunnelCode, String tunnelId) {
    }

    private record TunnelExpiration(int expirationHours, int expiresAt) {
    }
}
