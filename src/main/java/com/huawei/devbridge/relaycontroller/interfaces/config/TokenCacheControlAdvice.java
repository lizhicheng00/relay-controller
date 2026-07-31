package com.huawei.devbridge.relaycontroller.interfaces.config;

import com.huawei.devbridge.relaycontroller.interfaces.response.AuthTokenResponse;
import com.huawei.devbridge.relaycontroller.interfaces.response.TunnelTokenResponse;
import org.springframework.core.MethodParameter;
import org.springframework.http.CacheControl;
import org.springframework.http.MediaType;
import org.springframework.http.converter.HttpMessageConverter;
import org.springframework.http.server.ServerHttpRequest;
import org.springframework.http.server.ServerHttpResponse;
import org.springframework.web.bind.annotation.RestControllerAdvice;
import org.springframework.web.servlet.mvc.method.annotation.ResponseBodyAdvice;

@RestControllerAdvice
public class TokenCacheControlAdvice implements ResponseBodyAdvice<Object> {

    @Override
    public boolean supports(
            MethodParameter returnType, Class<? extends HttpMessageConverter<?>> converterType) {
        Class<?> responseType = returnType.getParameterType();
        return TunnelTokenResponse.class.isAssignableFrom(responseType)
                || AuthTokenResponse.class.isAssignableFrom(responseType);
    }

    @Override
    public Object beforeBodyWrite(
            Object body,
            MethodParameter returnType,
            MediaType selectedContentType,
            Class<? extends HttpMessageConverter<?>> selectedConverterType,
            ServerHttpRequest request,
            ServerHttpResponse response) {
        response.getHeaders().setCacheControl(CacheControl.noStore());
        return body;
    }
}
