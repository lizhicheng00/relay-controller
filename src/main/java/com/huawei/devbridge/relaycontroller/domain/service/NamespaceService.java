package com.huawei.devbridge.relaycontroller.domain.service;

import com.huawei.devbridge.relaycontroller.common.exception.BizException;
import com.huawei.devbridge.relaycontroller.common.exception.ErrorCode;
import com.huawei.devbridge.relaycontroller.common.validation.IdentifierValidator;
import com.huawei.devbridge.relaycontroller.domain.model.NamespaceContext;
import org.springframework.stereotype.Service;

@Service
public class NamespaceService {
    public String requireNamespace(String namespace) {
        return requireNamespace(namespace, "X-Namespace");
    }

    public NamespaceContext requireContext(String namespace, String accountNamespace) {
        return new NamespaceContext(
                requireNamespace(namespace),
                requireNamespace(accountNamespace, "X-Account-Namespace"));
    }

    private String requireNamespace(String namespace, String headerName) {
        if (namespace == null || namespace.isBlank()) {
            throw new BizException(ErrorCode.UNAUTHORIZED, headerName + " is required");
        }
        if (!IdentifierValidator.isValid(namespace)) {
            throw new BizException(ErrorCode.PARAM_INVALID, headerName + " is invalid");
        }
        return namespace;
    }
}
