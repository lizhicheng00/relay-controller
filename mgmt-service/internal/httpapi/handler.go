package httpapi

import (
	"context"
	"crypto/subtle"
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
	"mgmt-service/internal/security"
)

//go:embed openapi.yaml
var openAPISpec []byte

const apiBase = "/open-api-inner/v1/mgmt-service"

type identityContextKey struct{}

type API interface {
	IssueDefaultAPIKey(context.Context, core.IdentityAssertion, core.APIKeyType) (core.DefaultAPIKeyCredential, error)
	Authenticate(context.Context, string) (core.Identity, error)
	ListAPIKeys(context.Context, core.Identity) ([]core.APIKey, error)
	CreateAPIKey(context.Context, core.Identity, string, core.APIKeyType) (core.IssuedAPIKey, error)
	DeleteAPIKey(context.Context, core.Identity, string) error
}

type Handler struct {
	api               API
	trustedProxyToken string
	log               *slog.Logger
}

type issueDefaultAPIKeyRequest struct {
	Type core.APIKeyType `json:"type"`
}

type createAPIKeyRequest struct {
	Name string          `json:"name"`
	Type core.APIKeyType `json:"type"`
}

func New(api API, trustedProxyToken string, logger *slog.Logger) http.Handler {
	handler := &Handler{
		api: api, trustedProxyToken: trustedProxyToken, log: logger,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /openapi.yaml", handler.openapi)
	mux.HandleFunc("POST "+apiBase+"/api-keys/default", handler.issueDefaultAPIKey)
	mux.Handle("POST "+apiBase+"/api-keys/check", handler.authenticate(http.HandlerFunc(handler.checkAPIKey)))
	mux.Handle("GET "+apiBase+"/api-keys", handler.authenticate(http.HandlerFunc(handler.listAPIKeys)))
	mux.Handle("POST "+apiBase+"/api-keys", handler.authenticate(http.HandlerFunc(handler.createAPIKey)))
	mux.Handle("DELETE "+apiBase+"/api-keys/{keyId}", handler.authenticate(http.HandlerFunc(handler.deleteAPIKey)))
	return handler.recover(handler.requestContext(mux))
}

func (h *Handler) issueDefaultAPIKey(response http.ResponseWriter, request *http.Request) {
	if !constantTimeEqual(request.Header.Get("X-DevBridge-Proxy-Token"), h.trustedProxyToken) {
		h.writeError(response, core.Unauthorized("X-DevBridge-Proxy-Token"))
		return
	}
	var input issueDefaultAPIKeyRequest
	if err := decodeJSON(response, request, &input); err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.IssueDefaultAPIKey(request.Context(), core.IdentityAssertion{
		DomainID: request.Header.Get("X-Domain-Id"),
		UserID:   request.Header.Get("X-User-Id"),
	}, input.Type)
	if err != nil {
		h.writeError(response, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, result)
}

func (h *Handler) checkAPIKey(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, identityFromContext(request.Context()))
}

func (h *Handler) listAPIKeys(response http.ResponseWriter, request *http.Request) {
	keys, err := h.api.ListAPIKeys(request.Context(), identityFromContext(request.Context()))
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
		request.Context(), identityFromContext(request.Context()), input.Name, input.Type)
	if err != nil {
		h.writeError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, key)
}

func (h *Handler) deleteAPIKey(response http.ResponseWriter, request *http.Request) {
	err := h.api.DeleteAPIKey(
		request.Context(), identityFromContext(request.Context()), request.PathValue("keyId"))
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

func (h *Handler) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		identity, err := h.api.Authenticate(request.Context(), request.Header.Get("X-API-Key"))
		if err != nil {
			h.writeError(response, err)
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		ctx := context.WithValue(request.Context(), identityContextKey{}, identity)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func (h *Handler) requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Request-Id", security.NewID("req_"))
		response.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(response, request)
	})
}

func (h *Handler) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
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

func identityFromContext(ctx context.Context) core.Identity {
	identity, _ := ctx.Value(identityContextKey{}).(core.Identity)
	return identity
}

func constantTimeEqual(actual, expected string) bool {
	if len(actual) != len(expected) || len(actual) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
