package core

import (
	"errors"
	"fmt"
	"net/http"
)

type AppError struct {
	Status  int
	Code    string
	Message string
	Target  string
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Cause }

func AsAppError(err error) *AppError {
	var appError *AppError
	if errors.As(err, &appError) {
		return appError
	}
	return Internal("internal error", err)
}

func Invalid(target, message string) *AppError {
	return &AppError{
		Status: http.StatusBadRequest, Code: "PARAM_INVALID", Message: message, Target: target,
	}
}

func Unauthorized(target string) *AppError {
	return &AppError{
		Status: http.StatusUnauthorized, Code: "UNAUTHORIZED",
		Message: "authentication failed", Target: target,
	}
}

func Internal(message string, cause error) *AppError {
	return &AppError{
		Status: http.StatusInternalServerError, Code: "INTERNAL_ERROR",
		Message: message, Cause: cause,
	}
}
