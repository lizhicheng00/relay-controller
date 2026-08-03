package com.huawei.devbridge.relaycontroller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import com.huawei.clouds.wushan.scc.crypto.SccCrypto;
import com.huawei.devbridge.relaycontroller.infrastructure.config.RelayProperties;
import com.huawei.devbridge.relaycontroller.infrastructure.security.ApiKeyVerifier;
import org.junit.jupiter.api.Test;

class ApiKeyVerifierTest {
    private static final String PRIMARY_KEY = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
    private static final String STANDBY_KEY = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789";

    @Test
    void acceptsPrimaryAndStandbyKeysAfterDecryption() {
        RelayProperties properties = properties("encrypted-primary", "encrypted-standby");
        SccCrypto sccCrypto = mock(SccCrypto.class);
        when(sccCrypto.decrypt("encrypted-primary")).thenReturn(PRIMARY_KEY);
        when(sccCrypto.decrypt("encrypted-standby")).thenReturn(STANDBY_KEY);
        ApiKeyVerifier verifier = new ApiKeyVerifier(properties, sccCrypto);

        verifier.init();

        assertThat(verifier.matches(PRIMARY_KEY)).isTrue();
        assertThat(verifier.matches(STANDBY_KEY)).isTrue();
        assertThat(verifier.matches("invalid-api-key")).isFalse();
        assertThat(verifier.matches(null)).isFalse();
    }

    @Test
    void failsInitializationWithoutPrimaryKey() {
        ApiKeyVerifier verifier = new ApiKeyVerifier(properties(null, null), mock(SccCrypto.class));

        assertThatThrownBy(verifier::init)
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("API key primary");
    }

    @Test
    void disabledAuthenticationDoesNotRequireAKey() {
        RelayProperties properties = properties(null, null);
        properties.getSecurity().getApiKey().setEnabled(false);
        ApiKeyVerifier verifier = new ApiKeyVerifier(properties, mock(SccCrypto.class));

        verifier.init();

        assertThat(verifier.matches(null)).isTrue();
    }

    private static RelayProperties properties(String primary, String standby) {
        RelayProperties properties = new RelayProperties();
        properties.getSecurity().getApiKey().setPrimary(primary);
        properties.getSecurity().getApiKey().setStandby(standby);
        return properties;
    }
}
