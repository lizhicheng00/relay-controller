package com.huawei.devbridge.relaycontroller.interfaces.config;

import com.huawei.devbridge.relaycontroller.interfaces.rate.RateLimitInterceptor;
import lombok.RequiredArgsConstructor;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.servlet.config.annotation.InterceptorRegistry;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;

@Configuration
@RequiredArgsConstructor
public class WebMvcConfig implements WebMvcConfigurer {
    private static final String API_BASE = "/open-api-inner/v1/relay-controller";

    private final RateLimitInterceptor rateLimitInterceptor;

    @Override
    public void addInterceptors(InterceptorRegistry registry) {
        registry.addInterceptor(rateLimitInterceptor)
                .addPathPatterns(API_BASE + "/tunnels", API_BASE + "/tunnels/**", API_BASE + "/limits");
    }
}
