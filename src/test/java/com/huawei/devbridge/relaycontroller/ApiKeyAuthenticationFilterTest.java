package com.huawei.devbridge.relaycontroller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.huawei.devbridge.relaycontroller.infrastructure.security.ApiKeyVerifier;
import com.huawei.devbridge.relaycontroller.interfaces.security.ApiKeyAuthenticationFilter;
import jakarta.servlet.FilterChain;
import org.junit.jupiter.api.Test;
import org.springframework.http.HttpStatus;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;

class ApiKeyAuthenticationFilterTest {
    private static final String BASE = "/open-api-inner/v1/relay-controller";

    @Test
    void allowsNamespaceApiWithValidKey() throws Exception {
        ApiKeyVerifier verifier = mock(ApiKeyVerifier.class);
        when(verifier.matches("valid-key")).thenReturn(true);
        ApiKeyAuthenticationFilter filter = filter(verifier);
        MockHttpServletRequest request = new MockHttpServletRequest("GET", BASE + "/limits");
        request.addHeader(ApiKeyAuthenticationFilter.API_KEY_HEADER, "valid-key");
        MockHttpServletResponse response = new MockHttpServletResponse();
        FilterChain chain = mock(FilterChain.class);

        filter.doFilter(request, response, chain);

        verify(chain).doFilter(request, response);
    }

    @Test
    void rejectsNamespaceApiWithInvalidKey() throws Exception {
        ApiKeyVerifier verifier = mock(ApiKeyVerifier.class);
        ApiKeyAuthenticationFilter filter = filter(verifier);
        MockHttpServletRequest request = new MockHttpServletRequest("POST", BASE + "/tunnels");
        request.addHeader(ApiKeyAuthenticationFilter.API_KEY_HEADER, "invalid-key");
        MockHttpServletResponse response = new MockHttpServletResponse();
        FilterChain chain = mock(FilterChain.class);

        filter.doFilter(request, response, chain);

        assertThat(response.getStatus()).isEqualTo(HttpStatus.UNAUTHORIZED.value());
        assertThat(response.getContentAsString())
                .contains("\"code\":\"40100\"")
                .contains("\"target\":\"X-API-Key\"")
                .doesNotContain("invalid-key");
        verifyNoInteractions(chain);
    }

    @Test
    void leavesGatewayPolicyApiOnMtlsAuthentication() throws Exception {
        ApiKeyVerifier verifier = mock(ApiKeyVerifier.class);
        ApiKeyAuthenticationFilter filter = filter(verifier);
        MockHttpServletRequest request = new MockHttpServletRequest(
                "GET", BASE + "/clusters/cluster-a/tunnels/aaaadysa/ports/8080");
        MockHttpServletResponse response = new MockHttpServletResponse();
        FilterChain chain = mock(FilterChain.class);

        filter.doFilter(request, response, chain);

        verify(chain).doFilter(request, response);
        verifyNoInteractions(verifier);
    }

    private static ApiKeyAuthenticationFilter filter(ApiKeyVerifier verifier) {
        return new ApiKeyAuthenticationFilter(verifier, new ObjectMapper());
    }
}
