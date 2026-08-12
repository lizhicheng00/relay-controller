package httpapi

import (
	"context"
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"mgmt-service/internal/core"
	"mgmt-service/internal/security"
)

const maxRequestBody = 1 << 20

//go:embed openapi.yaml
var openAPISpec []byte

type authContextKey struct{}

type API interface {
	LoginIAM(context.Context, core.IAMIdentity) (core.LoginSession, error)
	IssueLoginAPIKey(context.Context, string, string, string) (core.IssuedAPIKey, error)
	Authenticate(context.Context, string) (core.AuthContext, error)

	ListAPIKeys(context.Context, core.Identity, string) ([]core.APIKey, error)
	CreateAPIKey(context.Context, core.Identity, string, string, string) (core.IssuedAPIKey, error)
	DeleteAPIKey(context.Context, core.Identity, string, string) error

	CreateNamespace(context.Context, core.Identity, string) (core.Namespace, error)
	GetNamespace(context.Context, core.Identity, string) (core.Namespace, error)
	ListNamespaces(context.Context, core.Identity) ([]core.Namespace, error)
	UpdateNamespace(context.Context, core.Identity, string, string) (core.Namespace, error)
	DeleteNamespace(context.Context, core.Identity, string) error
}

type Readiness interface {
	Ping(context.Context) error
}

type Handler struct {
	api               API
	dependencies      []Readiness
	trustedProxyToken string
	logger            *slog.Logger
	handler           http.Handler
}

type keyRequest struct {
	Name       string `json:"name"`
	Permission string `json:"permission"`
}

type namespaceRequest struct {
	DisplayName string `json:"displayName"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Target  string `json:"target,omitempty"`
}

func New(
	application API,
	dependencies []Readiness,
	trustedProxyToken string,
	logger *slog.Logger,
) *Handler {
	handler := &Handler{
		api: application, dependencies: dependencies,
		trustedProxyToken: trustedProxyToken, logger: logger,
	}
	handler.handler = handler.routes()
	return handler
}

func (s *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	s.handler.ServeHTTP(response, request)
}

func (s *Handler) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /openapi.yaml", s.openapi)
	mux.HandleFunc("POST /v1/auth/iam/login", s.loginIAM)
	mux.HandleFunc("POST /v1/auth/api-key", s.issueLoginAPIKey)

	mux.Handle("GET /v1/me", s.authenticate(core.PermissionRead, http.HandlerFunc(s.me)))
	mux.Handle("GET /v1/namespaces", s.authenticate(core.PermissionRead, http.HandlerFunc(s.listNamespaces)))
	mux.Handle("POST /v1/namespaces", s.authenticate(core.PermissionWrite, http.HandlerFunc(s.createNamespace)))
	mux.Handle("GET /v1/namespaces/{namespaceId}", s.authenticate(core.PermissionRead, http.HandlerFunc(s.getNamespace)))
	mux.Handle("PATCH /v1/namespaces/{namespaceId}", s.authenticate(core.PermissionWrite, http.HandlerFunc(s.updateNamespace)))
	mux.Handle("DELETE /v1/namespaces/{namespaceId}", s.authenticate(core.PermissionWrite, http.HandlerFunc(s.deleteNamespace)))
	mux.Handle("GET /v1/namespaces/{namespaceId}/api-keys", s.authenticate(core.PermissionRead, http.HandlerFunc(s.listAPIKeys)))
	mux.Handle("POST /v1/namespaces/{namespaceId}/api-keys", s.authenticate(core.PermissionWrite, http.HandlerFunc(s.createAPIKey)))
	mux.Handle("DELETE /v1/namespaces/{namespaceId}/api-keys/{keyId}", s.authenticate(core.PermissionWrite, http.HandlerFunc(s.deleteAPIKey)))
	return s.recoverPanic(s.requestContext(mux))
}

func (s *Handler) loginIAM(response http.ResponseWriter, request *http.Request) {
	if !constantTimeEqual(request.Header.Get("X-DevBridge-Proxy-Token"), s.trustedProxyToken) {
		writeError(response, http.StatusUnauthorized, "UNAUTHORIZED", "authentication failed", "")
		return
	}
	result, err := s.api.LoginIAM(request.Context(), core.IAMIdentity{
		DomainID: request.Header.Get("X-IAM-Domain-Id"),
		UserID:   request.Header.Get("X-IAM-User-Id"),
		UserName: request.Header.Get("X-IAM-User-Name"),
	})
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, result)
}

func (s *Handler) issueLoginAPIKey(response http.ResponseWriter, request *http.Request) {
	var input keyRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "PARAM_INVALID", err.Error(), "body")
		return
	}
	key, err := s.api.IssueLoginAPIKey(
		request.Context(), bearerToken(request.Header.Get("Authorization")),
		input.Name, input.Permission)
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusCreated, key)
}

func (s *Handler) me(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, authFromContext(request.Context()))
}

func (s *Handler) listNamespaces(response http.ResponseWriter, request *http.Request) {
	values, err := s.api.ListNamespaces(request.Context(), identityFromContext(request.Context()))
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusOK, values)
}

func (s *Handler) createNamespace(response http.ResponseWriter, request *http.Request) {
	var input namespaceRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "PARAM_INVALID", err.Error(), "body")
		return
	}
	value, err := s.api.CreateNamespace(
		request.Context(), identityFromContext(request.Context()), input.DisplayName)
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusCreated, value)
}

func (s *Handler) getNamespace(response http.ResponseWriter, request *http.Request) {
	value, err := s.api.GetNamespace(
		request.Context(), identityFromContext(request.Context()), request.PathValue("namespaceId"))
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Handler) updateNamespace(response http.ResponseWriter, request *http.Request) {
	var input namespaceRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "PARAM_INVALID", err.Error(), "body")
		return
	}
	value, err := s.api.UpdateNamespace(
		request.Context(), identityFromContext(request.Context()),
		request.PathValue("namespaceId"), input.DisplayName)
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Handler) deleteNamespace(response http.ResponseWriter, request *http.Request) {
	err := s.api.DeleteNamespace(
		request.Context(), identityFromContext(request.Context()), request.PathValue("namespaceId"))
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (s *Handler) listAPIKeys(response http.ResponseWriter, request *http.Request) {
	keys, err := s.api.ListAPIKeys(
		request.Context(), identityFromContext(request.Context()), request.PathValue("namespaceId"))
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusOK, keys)
}

func (s *Handler) createAPIKey(response http.ResponseWriter, request *http.Request) {
	var input keyRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "PARAM_INVALID", err.Error(), "body")
		return
	}
	key, err := s.api.CreateAPIKey(
		request.Context(), identityFromContext(request.Context()),
		request.PathValue("namespaceId"), input.Name, input.Permission)
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusCreated, key)
}

func (s *Handler) deleteAPIKey(response http.ResponseWriter, request *http.Request) {
	err := s.api.DeleteAPIKey(
		request.Context(), identityFromContext(request.Context()),
		request.PathValue("namespaceId"), request.PathValue("keyId"))
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (s *Handler) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Handler) ready(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	for _, dependency := range s.dependencies {
		if err := dependency.Ping(ctx); err != nil {
			writeError(response, http.StatusServiceUnavailable, "NOT_READY", "service is not ready", "")
			return
		}
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Handler) openapi(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	response.Header().Set("Cache-Control", "public, max-age=300")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(openAPISpec)
}

func (s *Handler) authenticate(required string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		auth, err := s.api.Authenticate(request.Context(), request.Header.Get("X-API-Key"))
		if err != nil {
			s.handleError(response, request, err)
			return
		}
		if required == core.PermissionWrite && auth.Permission != core.PermissionWrite {
			writeError(response, http.StatusForbidden, "FORBIDDEN", "write permission is required", "X-API-Key")
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		ctx := context.WithValue(request.Context(), authContextKey{}, auth)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func (s *Handler) requestContext(next http.Handler) http.Handler {
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

func (s *Handler) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("request panic",
					"method", request.Method, "path", request.URL.Path,
					"error", recovered, "stack", string(debug.Stack()))
				writeError(response, http.StatusInternalServerError,
					"INTERNAL_ERROR", "internal server error", "")
			}
		}()
		next.ServeHTTP(response, request)
	})
}

func (s *Handler) handleError(response http.ResponseWriter, request *http.Request, err error) {
	applicationError := core.AsAppError(err)
	message := applicationError.Message
	if applicationError.Status >= http.StatusInternalServerError {
		s.logger.Error("request failed",
			"method", request.Method, "path", request.URL.Path, "error", err)
		message = "internal server error"
	}
	writeError(response, applicationError.Status, applicationError.Code, message, applicationError.Target)
}

func decodeJSON(response http.ResponseWriter, request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(response, request.Body, maxRequestBody)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeError(response http.ResponseWriter, status int, code, message, target string) {
	writeJSON(response, status, errorEnvelope{Error: errorBody{
		Code: code, Message: message, Target: target,
	}})
}

func authFromContext(ctx context.Context) core.AuthContext {
	auth, _ := ctx.Value(authContextKey{}).(core.AuthContext)
	return auth
}

func identityFromContext(ctx context.Context) core.Identity {
	return authFromContext(ctx).Identity
}

func bearerToken(header string) string {
	scheme, token, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func constantTimeEqual(actual, expected string) bool {
	if len(actual) != len(expected) || len(actual) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
