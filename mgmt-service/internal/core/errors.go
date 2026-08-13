package core

import (
	"errors"
	"fmt"
	"net/http"
)

const (
	CodeParamInvalid       = "40000"
	CodeUnauthorized       = "40100"
	CodeAPIKeyNotFound     = "13001"
	CodeAPIKeyNameConflict = "13002"
	CodeAPIKeyLimitReached = "13003"
	CodeDefaultAPIKey      = "13004"
	CodeInternal           = "50000"
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
	return ErrorResponse{Error: ErrorBody{
		Code: e.Code, Message: e.Message, Target: e.Target, Details: e.Details,
	}}
}

func AsAppError(err error) *AppError {
	var appError *AppError
	if errors.As(err, &appError) {
		return appError
	}
	return Internal(err)
}

func Invalid(target, message string) *AppError {
	return &AppError{
		Status: http.StatusBadRequest, Code: CodeParamInvalid,
		Message: "request validation failed",
		Details: []ErrorDetail{{Code: CodeParamInvalid, Target: target, Message: message}},
	}
}

func Unauthorized(target string) *AppError {
	return &AppError{
		Status: http.StatusUnauthorized, Code: CodeUnauthorized,
		Message: "authentication failed", Target: target,
	}
}

func NotFound(target, message string) *AppError {
	return &AppError{
		Status: http.StatusNotFound, Code: CodeAPIKeyNotFound, Message: message, Target: target,
	}
}

func Conflict(code, target, message string) *AppError {
	return &AppError{
		Status: http.StatusConflict, Code: code, Message: message, Target: target,
	}
}

func Internal(cause error) *AppError {
	return &AppError{
		Status: http.StatusInternalServerError, Code: CodeInternal,
		Message: "internal error", Cause: cause,
	}
}
