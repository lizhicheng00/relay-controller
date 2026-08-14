package core

import (
	"errors"
	"testing"
)

func TestInvalidResponse(t *testing.T) {
	response := Invalid("type", "is invalid").Response()
	if response.Error.Code != CodeParamInvalid || response.Error.Message != "request validation failed" ||
		len(response.Error.Details) != 1 || response.Error.Details[0].Target != "type" {
		t.Fatalf("response = %#v", response)
	}
}

func TestUnknownErrorIsInternal(t *testing.T) {
	appError := AsAppError(errors.New("database unavailable"))
	if appError.Code != CodeInternal || appError.Status != 500 || appError.Cause == nil {
		t.Fatalf("appError = %#v", appError)
	}
}
