package service

import "fmt"

type ErrorKind string

const (
	KindInvalid      ErrorKind = "invalid"
	KindUnauthorized ErrorKind = "unauthorized"
	KindNotFound     ErrorKind = "not_found"
	KindConflict     ErrorKind = "conflict"
	KindInternal     ErrorKind = "internal"
)

type Error struct {
	Kind    ErrorKind
	Code    string
	Message string
	Target  string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func invalid(target, message string) error {
	return &Error{Kind: KindInvalid, Code: "PARAM_INVALID", Message: message, Target: target}
}

func unauthorizedTarget(target string) error {
	return &Error{
		Kind:    KindUnauthorized,
		Code:    "UNAUTHORIZED",
		Message: "authentication failed",
		Target:  target,
	}
}

func internal(operation string, cause error) error {
	return &Error{
		Kind:    KindInternal,
		Code:    "INTERNAL_ERROR",
		Message: operation,
		Cause:   cause,
	}
}
