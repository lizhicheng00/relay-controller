package httpapi

import (
	"context"
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"mgmt-service/internal/core"
	"mgmt-service/internal/security"
)

//go:embed openapi.yaml
var openAPISpec []byte

type identityContextKey struct{}

type API interface {
	ProvisionAPIKey(context.Context, core.IdentityAssertion) (core.ProvisionedCredential, error)
	Authenticate(context.Context, string) (core.Identity, error)
}

type Readiness interface {
	Ping(context.Context) error
}

type Handler struct {
	api               API
	dependencies      []Readiness
	trustedProxyToken string
	log               *slog.Logger
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Target  string `json:"target,omitempty"`
}

func New(api API, dependencies []Readiness, trustedProxyToken string, logger *slog.Logger) http.Handler {
	handler := &Handler{
		api: api, dependencies: dependencies, trustedProxyToken: trustedProxyToken, log: logger,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handler.health)
	mux.HandleFunc("GET /readyz", handler.ready)
	mux.HandleFunc("GET /openapi.yaml", handler.openapi)
	mux.HandleFunc("POST /v1/api-key", handler.provisionAPIKey)
	mux.Handle("GET /v1/me", handler.authenticate(http.HandlerFunc(handler.me)))
	return handler.recover(handler.requestContext(mux))
}

func (h *Handler) provisionAPIKey(response http.ResponseWriter, request *http.Request) {
	if !constantTimeEqual(request.Header.Get("X-DevBridge-Proxy-Token"), h.trustedProxyToken) {
		writeError(response, core.Unauthorized("X-DevBridge-Proxy-Token"))
		return
	}
	result, err := h.api.ProvisionAPIKey(request.Context(), core.IdentityAssertion{
		DomainID: request.Header.Get("X-Domain-Id"),
		UserID:   request.Header.Get("X-User-Id"),
	})
	if err != nil {
		h.writeError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, result)
}

func (h *Handler) me(response http.ResponseWriter, request *http.Request) {
	identity, _ := request.Context().Value(identityContextKey{}).(core.Identity)
	writeJSON(response, http.StatusOK, identity)
}

func (h *Handler) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ready(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	for _, dependency := range h.dependencies {
		if err := dependency.Ping(ctx); err != nil {
			writeError(response, &core.AppError{
				Status: http.StatusServiceUnavailable, Code: "NOT_READY", Message: "service is not ready",
			})
			return
		}
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
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
			h.writeError(response, request, err)
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		ctx := context.WithValue(request.Context(), identityContextKey{}, identity)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func (h *Handler) requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestID, err := security.NewID("req_")
		if err != nil {
			requestID = "unavailable"
		}
		response.Header().Set("X-Request-Id", requestID)
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
				writeError(response, core.Internal("internal server error", nil))
			}
		}()
		next.ServeHTTP(response, request)
	})
}

func (h *Handler) writeError(response http.ResponseWriter, request *http.Request, err error) {
	appError := core.AsAppError(err)
	message := appError.Message
	if appError.Status >= http.StatusInternalServerError {
		h.log.Error("request failed", "method", request.Method, "path", request.URL.Path, "error", err)
		message = "internal server error"
	}
	writeJSON(response, appError.Status, errorEnvelope{Error: errorBody{
		Code: appError.Code, Message: message, Target: appError.Target,
	}})
}

func writeError(response http.ResponseWriter, err *core.AppError) {
	writeJSON(response, err.Status, errorEnvelope{Error: errorBody{
		Code: err.Code, Message: err.Message, Target: err.Target,
	}})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func constantTimeEqual(actual, expected string) bool {
	if len(actual) != len(expected) || len(actual) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
