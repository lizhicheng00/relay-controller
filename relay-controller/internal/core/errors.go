package core

import (
	"errors"
	"fmt"
	"net/http"
)

const (
	CodeParamInvalid            = "40000"
	CodeUnauthorized            = "40100"
	CodeClusterNotFound         = "10001"
	CodeTunnelNotFound          = "10002"
	CodeTunnelIDConflict        = "10003"
	CodeTunnelExpired           = "10004"
	CodeTunnelAccessDenied      = "10005"
	CodeTunnelQuotaExceeded     = "10006"
	CodeTunnelNameConflict      = "10007"
	CodeTunnelPortInvalid       = "11001"
	CodeTunnelPortExists        = "11002"
	CodeTunnelPortNotFound      = "11003"
	CodeTunnelPortQuotaExceeded = "11005"
	CodeAccountDisabled         = "12001"
	CodeAccountQuotaExceeded    = "12002"
	CodeJWTGenerateFailed       = "30001"
	CodeRateLimited             = "42900"
	CodeInternal                = "50000"
)

type AppError struct {
	Status  int
	Code    string
	Message string
	Target  string
	Details []ErrorDetail
	Cause   error
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Target  string        `json:"target,omitempty"`
	Details []ErrorDetail `json:"details,omitempty"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Target  string `json:"target"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Cause }

func (e *AppError) Response() ErrorResponse {
	return ErrorResponse{Error: ErrorBody{Code: e.Code, Message: e.Message, Target: e.Target, Details: e.Details}}
}

func AsAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal(err)
}

func NewError(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

func Invalid(message string) *AppError {
	return NewError(http.StatusBadRequest, CodeParamInvalid, message)
}

func InvalidField(target, message string) *AppError {
	return &AppError{
		Status: http.StatusBadRequest, Code: CodeParamInvalid, Message: "request validation failed",
		Details: []ErrorDetail{{Code: CodeParamInvalid, Target: target, Message: message}},
	}
}

func MissingHeader(name string) *AppError {
	return &AppError{Status: http.StatusUnauthorized, Code: CodeUnauthorized, Message: name + " is required", Target: name}
}

func Conflict(code, target, message string) *AppError {
	return &AppError{Status: http.StatusConflict, Code: code, Message: message, Target: target}
}

func Internal(cause error) *AppError {
	return &AppError{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "internal error", Cause: cause}
}
