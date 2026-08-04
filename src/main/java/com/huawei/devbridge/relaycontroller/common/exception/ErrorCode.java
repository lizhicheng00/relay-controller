package com.huawei.devbridge.relaycontroller.common.exception;

import lombok.Getter;

@Getter
public enum ErrorCode {
    PARAM_INVALID("40000", "parameter invalid"),
    UNAUTHORIZED("40100", "unauthorized"),

    CLUSTER_NOT_FOUND("10001", "cluster not found"),
    TUNNEL_NOT_FOUND("10002", "tunnel not found"),
    TUNNEL_ID_CONFLICT("10003", "tunnel id conflict"),
    TUNNEL_EXPIRED("10004", "tunnel expired"),
    TUNNEL_ACCESS_DENIED("10005", "tunnel access denied"),
    TUNNEL_QUOTA_EXCEEDED("10006", "tunnel quota exceeded"),
    TUNNEL_PORT_INVALID("11001", "tunnel port invalid"),
    TUNNEL_PORT_ALREADY_EXISTS("11002", "tunnel port already exists"),
    TUNNEL_PORT_NOT_FOUND("11003", "tunnel port not found"),
    TUNNEL_PORT_QUOTA_EXCEEDED("11005", "tunnel port quota exceeded"),

    ACCOUNT_DISABLED("12001", "account disabled"),
    ACCOUNT_QUOTA_EXCEEDED("12002", "monthly traffic quota exceeded"),

    JWT_GENERATE_FAILED("30001", "jwt generate failed"),
    JWT_KEY_INVALID("30002", "jwt key invalid"),

    RATE_LIMITED("42900", "rate limited"),

    INTERNAL_ERROR("50000", "internal error");

    private final String code;
    private final String message;

    ErrorCode(String code, String message) {
        this.code = code;
        this.message = message;
    }
}
