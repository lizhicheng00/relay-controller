package com.huawei.devbridge.relaycontroller.infrastructure.security;

import com.huawei.clouds.wushan.scc.crypto.SccCrypto;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import jakarta.annotation.PostConstruct;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class ApiKeyVerifier {
    private static final int MIN_API_KEY_LENGTH = 32;

    private final RelayProperties relayProperties;
    private final SccCrypto sccCrypto;
    private boolean enabled;
    private byte[] primaryHash;
    private byte[] standbyHash;

    @PostConstruct
    public void init() {
        RelayProperties.ApiKey apiKey = relayProperties.getSecurity().getApiKey();
        enabled = apiKey.isEnabled();
        if (!enabled) {
            return;
        }
        primaryHash = hash(requireStrongKey(decrypt(apiKey.getPrimary()), "primary"));
        String standby = decrypt(apiKey.getStandby());
        if (standby != null && !standby.isBlank()) {
            standbyHash = hash(requireStrongKey(standby, "standby"));
        }
    }

    public boolean matches(String candidate) {
        if (!enabled) {
            return true;
        }
        if (candidate == null || candidate.isBlank()) {
            return false;
        }
        byte[] candidateHash = hash(candidate);
        boolean primaryMatches = MessageDigest.isEqual(primaryHash, candidateHash);
        boolean standbyMatches = standbyHash != null && MessageDigest.isEqual(standbyHash, candidateHash);
        return primaryMatches || standbyMatches;
    }

    private String decrypt(String configuredKey) {
        return configuredKey == null || configuredKey.isBlank() ? configuredKey : sccCrypto.decrypt(configuredKey);
    }

    private static String requireStrongKey(String key, String name) {
        if (key == null || key.length() < MIN_API_KEY_LENGTH) {
            throw new IllegalStateException("relay security API key " + name
                    + " must contain at least " + MIN_API_KEY_LENGTH + " characters");
        }
        return key;
    }

    private static byte[] hash(String value) {
        try {
            return MessageDigest.getInstance("SHA-256")
                    .digest(value.getBytes(StandardCharsets.UTF_8));
        } catch (NoSuchAlgorithmException exception) {
            throw new IllegalStateException("SHA-256 is unavailable", exception);
        }
    }
}
