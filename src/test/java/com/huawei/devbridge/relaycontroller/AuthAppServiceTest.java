package com.huawei.devbridge.relaycontroller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.huawei.devbridge.relaycontroller.application.service.AuthAppService;
import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.domain.model.JwtToken;
import com.huawei.devbridge.relaycontroller.domain.service.JwtTokenService;
import com.huawei.devbridge.relaycontroller.domain.service.NamespaceService;
import com.huawei.devbridge.relaycontroller.interfaces.response.AuthTokenResponse;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
class AuthAppServiceTest {
    @Mock
    private JwtTokenService jwtTokenService;

    @Test
    void issuesNamespaceTokenWithStableResponseShape() {
        AuthAppService service = new AuthAppService(new NamespaceService(), jwtTokenService);
        when(jwtTokenService.issueAuthToken("ns-1"))
                .thenReturn(new JwtToken("auth-token", 3600L, 1785500000L));

        AuthTokenResponse response = service.issueToken("ns-1");

        assertThat(response.getNamespace()).isEqualTo("ns-1");
        assertThat(response.getScope()).isEqualTo("devbridge");
        assertThat(response.getLifetime()).isEqualTo(3600L);
        assertThat(response.getExpiration()).isEqualTo(1785500000L);
        assertThat(response.getToken()).isEqualTo("auth-token");
        verify(jwtTokenService).issueAuthToken("ns-1");
    }

    @Test
    void rejectsInvalidNamespaceBeforeSigning() {
        AuthAppService service = new AuthAppService(new NamespaceService(), jwtTokenService);

        assertThatThrownBy(() -> service.issueToken("invalid namespace"))
                .isInstanceOf(BizException.class)
                .extracting("errorCode")
                .isEqualTo(ErrorCode.PARAM_INVALID);
    }
}
