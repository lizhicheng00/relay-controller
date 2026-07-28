package com.huawei.devbridge.relaycontroller.infrastructure.config;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

@Data
@Component
@ConfigurationProperties(prefix = "relay")
public class RelayProperties {
    private String domain;
    private String region = "region-a";
    private int defaultExpirationHours = 72;
    private Tunnel tunnel = new Tunnel();
    private RateLimit rateLimit = new RateLimit();
    private Jwt jwt = new Jwt();
    private Billing billing = new Billing();

    @Data
    public static class Tunnel {
        private int cleanupRetentionDays = 3;
        private long cleanupInitialDelayMs = 3600000;
        private long cleanupIntervalMs = 3600000;
    }

    @Data
    public static class RateLimit {
        private boolean enabled = true;
        private int requestsPerMinute;
    }

    @Data
    public static class Jwt {
        private String issuer = "devbridge";
        private String audience = "relay-gateway";
        private String keyId = "1";
        private String privateKey;
        private TokenTtl token = new TokenTtl(86400);
    }

    @Data
    public static class Billing {
        private String defaultPlanCode = "trial";
        private boolean enforcementEnabled = true;
        private boolean settlementEnabled = true;
        private String settlementCron = "0 */10 * * * *";
        private int settlementBatchSize = 500;
        private int statusReportIntervalSeconds = 10;
        private int activityRefreshIntervalSeconds = 300;
        private int statusRetentionSeconds = 86400;
        private int statusMaxClockSkewSeconds = 300;
    }

    @Data
    public static class TokenTtl {
        private long ttlSeconds;

        public TokenTtl() {
        }

        public TokenTtl(long ttlSeconds) {
            this.ttlSeconds = ttlSeconds;
        }
    }
}
