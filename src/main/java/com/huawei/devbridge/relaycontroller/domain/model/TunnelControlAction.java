package com.huawei.devbridge.relaycontroller.domain.model;

import com.fasterxml.jackson.annotation.JsonValue;

public enum TunnelControlAction {
    KEEP("keep"),
    DISCONNECT("disconnect");

    private final String value;

    TunnelControlAction(String value) {
        this.value = value;
    }

    @JsonValue
    public String value() {
        return value;
    }
}
