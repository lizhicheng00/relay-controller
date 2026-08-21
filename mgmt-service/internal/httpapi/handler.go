package httpapi

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"runtime/debug"
	"strings"

	"mgmt-service/internal/core"
)

//go:embed openapi.yaml
var openAPISpec []byte

const apiBase = "/open-api-inner/v1/mgmt-service"

type API interface {
	CheckAPIKey(context.Context, string) (core.APIKeyIdentity, error)
	ListAPIKeys(context.Context, core.IdentityAssertion) ([]core.APIKey, error)
	CreateAPIKey(context.Context, core.IdentityAssertion, string, core.APIKeyScope) (core.IssuedAPIKey, error)
	DeleteAPIKey(context.Context, core.IdentityAssertion, string) error
}

type Handler struct {
	api API
	log *slog.Logger
}

type createAPIKeyRequest struct {
	Name  string           `json:"name"`
	Scope core.APIKeyScope `json:"scope"`
}

func New(api API, logger *slog.Logger) http.Handler {
	handler := &Handler{api: api, log: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /openapi.yaml", handler.openapi)
	mux.HandleFunc("POST "+apiBase+"/api-keys/check", handler.checkAPIKey)
	mux.HandleFunc("GET "+apiBase+"/api-keys", handler.listAPIKeys)
	mux.HandleFunc("POST "+apiBase+"/api-keys", handler.createAPIKey)
	mux.HandleFunc("DELETE "+apiBase+"/api-keys/{keyId}", handler.deleteAPIKey)
	return handler.recover(mux)
}

func (h *Handler) checkAPIKey(response http.ResponseWriter, request *http.Request) {
	identity, err := h.api.CheckAPIKey(request.Context(), request.Header.Get("X-API-Key"))
	if err != nil {
		h.writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, identity)
}

func (h *Handler) listAPIKeys(response http.ResponseWriter, request *http.Request) {
	keys, err := h.api.ListAPIKeys(request.Context(), identityAssertion(request))
	if err != nil {
		h.writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, keys)
}

func (h *Handler) createAPIKey(response http.ResponseWriter, request *http.Request) {
	var input createAPIKeyRequest
	if err := decodeJSON(response, request, &input); err != nil {
		h.writeError(response, err)
		return
	}
	key, err := h.api.CreateAPIKey(
		request.Context(), identityAssertion(request), input.Name, input.Scope)
	if err != nil {
		h.writeError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, key)
}

func (h *Handler) deleteAPIKey(response http.ResponseWriter, request *http.Request) {
	err := h.api.DeleteAPIKey(
		request.Context(), identityAssertion(request), request.PathValue("keyId"))
	if err != nil {
		h.writeError(response, err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (h *Handler) openapi(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	response.Header().Set("Cache-Control", "public, max-age=300")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(openAPISpec)
}

func (h *Handler) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Cache-Control", "no-store")
		defer func() {
			if recovered := recover(); recovered != nil {
				h.log.Error("request panic", "method", request.Method, "path", request.URL.Path,
					"error", recovered, "stack", string(debug.Stack()))
				writeJSON(response, http.StatusInternalServerError, core.Internal(nil).Response())
			}
		}()
		next.ServeHTTP(response, request)
	})
}

func (h *Handler) writeError(response http.ResponseWriter, err error) {
	appError := core.AsAppError(err)
	if appError.Status >= http.StatusInternalServerError {
		h.log.Error("request failed", "error", appError.Error(), "stack", string(debug.Stack()))
	}
	writeJSON(response, appError.Status, appError.Response())
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func decodeJSON(response http.ResponseWriter, request *http.Request, value any) error {
	if contentType := request.Header.Get("Content-Type"); contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || !strings.EqualFold(mediaType, "application/json") {
			return core.Invalid("requestBody", "request body is invalid")
		}
	}
	request.Body = http.MaxBytesReader(response, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return &core.AppError{
			Status: http.StatusBadRequest, Code: core.CodeParamInvalid,
			Message: "request body is invalid", Target: "requestBody", Cause: err,
		}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &core.AppError{
			Status: http.StatusBadRequest, Code: core.CodeParamInvalid,
			Message: "request body is invalid", Target: "requestBody", Cause: err,
		}
	}
	return nil
}

func identityAssertion(request *http.Request) core.IdentityAssertion {
	return core.IdentityAssertion{
		DomainID: request.Header.Get("X-Domain-Id"),
		UserID:   request.Header.Get("X-User-Id"),
	}
}
