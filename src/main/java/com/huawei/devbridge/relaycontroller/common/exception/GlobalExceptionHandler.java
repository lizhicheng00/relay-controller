package com.huawei.devbridge.relaycontroller.common.exception;

import com.huaweicloud.cloudspace.commons.framework.utils.ExceptionUtils;
import com.huawei.devbridge.relaycontroller.common.model.ErrorResponse;
import com.huawei.devbridge.relaycontroller.common.model.ErrorResponse.ErrorDetail;
import jakarta.validation.ConstraintViolationException;
import java.util.List;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.validation.BindException;
import org.springframework.web.bind.MissingRequestHeaderException;
import org.springframework.web.bind.MissingServletRequestParameterException;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

@RestControllerAdvice
@Slf4j
public class GlobalExceptionHandler {

    @ExceptionHandler(BizException.class)
    public ResponseEntity<ErrorResponse> handleBizException(BizException exception) {
        log.warn("Business exception: code={}, message={}",
                exception.getErrorCode().getCode(), exception.getMessage());
        return failure(statusOf(exception.getErrorCode()), exception.getErrorCode(), exception.getMessage());
    }

    @ExceptionHandler({MethodArgumentNotValidException.class, BindException.class})
    public ResponseEntity<ErrorResponse> handleBindException(BindException exception) {
        List<ErrorDetail> details = exception.getBindingResult().getFieldErrors().stream()
                .map(error -> new ErrorDetail(
                        ErrorCode.PARAM_INVALID.getCode(), error.getField(), error.getDefaultMessage()))
                .toList();
        log.warn("Request validation failed: {} field(s)", details.size());
        return ResponseEntity.badRequest().body(ErrorResponse.validation("request validation failed", details));
    }

    @ExceptionHandler(ConstraintViolationException.class)
    public ResponseEntity<ErrorResponse> handleConstraintViolation(ConstraintViolationException exception) {
        List<ErrorDetail> details = exception.getConstraintViolations().stream()
                .map(violation -> new ErrorDetail(ErrorCode.PARAM_INVALID.getCode(),
                        lastPathSegment(violation.getPropertyPath().toString()), violation.getMessage()))
                .toList();
        log.warn("Request constraint violation: {}", ExceptionUtils.anonymousMessage(exception));
        return ResponseEntity.badRequest().body(ErrorResponse.validation("request validation failed", details));
    }

    @ExceptionHandler(MissingRequestHeaderException.class)
    public ResponseEntity<ErrorResponse> handleMissingRequestHeader(MissingRequestHeaderException exception) {
        log.warn("Missing request header: {}", exception.getHeaderName());
        if ("X-Namespace".equalsIgnoreCase(exception.getHeaderName())
                || "X-Account-Namespace".equalsIgnoreCase(exception.getHeaderName())) {
            return failure(HttpStatus.UNAUTHORIZED, ErrorCode.UNAUTHORIZED,
                    exception.getHeaderName() + " is required", exception.getHeaderName());
        }
        return failure(HttpStatus.BAD_REQUEST, ErrorCode.PARAM_INVALID,
                "required request header is missing", exception.getHeaderName());
    }

    @ExceptionHandler(MissingServletRequestParameterException.class)
    public ResponseEntity<ErrorResponse> handleMissingRequestParameter(
            MissingServletRequestParameterException exception) {
        log.warn("Missing request parameter: {}", exception.getParameterName());
        return failure(HttpStatus.BAD_REQUEST, ErrorCode.PARAM_INVALID,
                "required request parameter is missing", exception.getParameterName());
    }

    @ExceptionHandler(HttpMessageNotReadableException.class)
    public ResponseEntity<ErrorResponse> handleMessageNotReadable(HttpMessageNotReadableException exception) {
        log.warn("Request body not readable: {}", ExceptionUtils.anonymousMessage(exception));
        return failure(HttpStatus.BAD_REQUEST, ErrorCode.PARAM_INVALID,
                "request body is invalid", "requestBody");
    }

    @ExceptionHandler(RuntimeException.class)
    public ResponseEntity<ErrorResponse> handleRuntimeException(RuntimeException exception) {
        log.error("Unhandled exception: {}", ExceptionUtils.anonymousMessage(exception));
        return failure(HttpStatus.INTERNAL_SERVER_ERROR, ErrorCode.INTERNAL_ERROR);
    }

    private static ResponseEntity<ErrorResponse> failure(HttpStatus status, ErrorCode errorCode) {
        return ResponseEntity.status(status).body(ErrorResponse.of(errorCode));
    }

    private static ResponseEntity<ErrorResponse> failure(
            HttpStatus status, ErrorCode errorCode, String message) {
        return failure(status, errorCode, message, null);
    }

    private static ResponseEntity<ErrorResponse> failure(
            HttpStatus status, ErrorCode errorCode, String message, String target) {
        return ResponseEntity.status(status).body(ErrorResponse.of(errorCode, message, target));
    }

    private static HttpStatus statusOf(ErrorCode errorCode) {
        return switch (errorCode) {
            case PARAM_INVALID, TUNNEL_PORT_INVALID -> HttpStatus.BAD_REQUEST;
            case UNAUTHORIZED -> HttpStatus.UNAUTHORIZED;
            case TUNNEL_ACCESS_DENIED, ACCOUNT_DISABLED -> HttpStatus.FORBIDDEN;
            case CLUSTER_NOT_FOUND, TUNNEL_NOT_FOUND, TUNNEL_PORT_NOT_FOUND -> HttpStatus.NOT_FOUND;
            case TUNNEL_EXPIRED -> HttpStatus.GONE;
            case TUNNEL_ID_CONFLICT, TUNNEL_PORT_ALREADY_EXISTS -> HttpStatus.CONFLICT;
            case TUNNEL_QUOTA_EXCEEDED, TUNNEL_PORT_QUOTA_EXCEEDED,
                    ACCOUNT_QUOTA_EXCEEDED, RATE_LIMITED -> HttpStatus.TOO_MANY_REQUESTS;
            case JWT_GENERATE_FAILED, JWT_KEY_INVALID, INTERNAL_ERROR -> HttpStatus.INTERNAL_SERVER_ERROR;
        };
    }

    private static String lastPathSegment(String path) {
        int separator = path.lastIndexOf('.');
        return separator < 0 ? path : path.substring(separator + 1);
    }
}
