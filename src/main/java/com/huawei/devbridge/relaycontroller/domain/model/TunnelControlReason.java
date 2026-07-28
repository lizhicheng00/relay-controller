package com.huawei.devbridge.relaycontroller.domain.model;

import com.fasterxml.jackson.annotation.JsonValue;

public enum TunnelControlReason {
    NONE("none"),
    TUNNEL_NOT_FOUND("tunnel_not_found"),
    CLUSTER_MISMATCH("cluster_mismatch"),
    TUNNEL_EXPIRED("tunnel_expired"),
    ACCOUNT_UNAVAILABLE("account_unavailable"),
    ACCOUNT_DISABLED("account_disabled"),
    QUOTA_EXCEEDED("quota_exceeded"),
    HOST_LIMIT_EXCEEDED("host_limit_exceeded");

    private final String value;

    TunnelControlReason(String value) {
        this.value = value;
    }

    @JsonValue
    public String value() {
        return value;
    }
}
