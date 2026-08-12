package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"mgmt-service/internal/domain"
	"mgmt-service/internal/idgen"
	"mgmt-service/internal/service"
)

const maxRequestBody = 1 << 20

type authContextKey struct{}

type Application interface {
	LoginIAM(context.Context, domain.IAMIdentity) (domain.LoginSession, error)
	IssueLoginAPIKey(context.Context, string, string, string) (domain.IssuedAPIKey, error)
	Authenticate(context.Context, string) (domain.AuthContext, error)

	ListAPIKeys(context.Context, domain.Identity, string) ([]domain.APIKey, error)
	CreateAPIKey(context.Context, domain.Identity, string, string, string) (domain.IssuedAPIKey, error)
	DeleteAPIKey(context.Context, domain.Identity, string, string) error

	CreateNamespace(context.Context, domain.Identity, string) (domain.Namespace, error)
	GetNamespace(context.Context, domain.Identity, string) (domain.Namespace, error)
	ListNamespaces(context.Context, domain.Identity) ([]domain.Namespace, error)
	UpdateNamespace(context.Context, domain.Identity, string, string) (domain.Namespace, error)
	DeleteNamespace(context.Context, domain.Identity, string) error
}

type Readiness interface {
	Ping(context.Context) error
}

type Server struct {
	service           Application
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

func NewServer(
	application Application,
	dependencies []Readiness,
	trustedProxyToken string,
	logger *slog.Logger,
) *Server {
	server := &Server{
		service: application, dependencies: dependencies,
		trustedProxyToken: trustedProxyToken, logger: logger,
	}
	server.handler = server.routes()
	return server
}

func (s *Server) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	s.handler.ServeHTTP(response, request)
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /openapi.yaml", s.openapi)
	mux.HandleFunc("POST /v1/auth/iam/login", s.loginIAM)
	mux.HandleFunc("POST /v1/auth/api-key", s.issueLoginAPIKey)

	mux.Handle("GET /v1/me", s.authenticate(domain.PermissionRead, http.HandlerFunc(s.me)))
	mux.Handle("GET /v1/namespaces", s.authenticate(domain.PermissionRead, http.HandlerFunc(s.listNamespaces)))
	mux.Handle("POST /v1/namespaces", s.authenticate(domain.PermissionWrite, http.HandlerFunc(s.createNamespace)))
	mux.Handle("GET /v1/namespaces/{namespaceId}", s.authenticate(domain.PermissionRead, http.HandlerFunc(s.getNamespace)))
	mux.Handle("PATCH /v1/namespaces/{namespaceId}", s.authenticate(domain.PermissionWrite, http.HandlerFunc(s.updateNamespace)))
	mux.Handle("DELETE /v1/namespaces/{namespaceId}", s.authenticate(domain.PermissionWrite, http.HandlerFunc(s.deleteNamespace)))
	mux.Handle("GET /v1/namespaces/{namespaceId}/api-keys", s.authenticate(domain.PermissionRead, http.HandlerFunc(s.listAPIKeys)))
	mux.Handle("POST /v1/namespaces/{namespaceId}/api-keys", s.authenticate(domain.PermissionWrite, http.HandlerFunc(s.createAPIKey)))
	mux.Handle("DELETE /v1/namespaces/{namespaceId}/api-keys/{keyId}", s.authenticate(domain.PermissionWrite, http.HandlerFunc(s.deleteAPIKey)))
	return s.recoverPanic(s.requestContext(mux))
}

func (s *Server) loginIAM(response http.ResponseWriter, request *http.Request) {
	if !constantTimeEqual(request.Header.Get("X-DevBridge-Proxy-Token"), s.trustedProxyToken) {
		writeError(response, http.StatusUnauthorized, "UNAUTHORIZED", "authentication failed", "")
		return
	}
	result, err := s.service.LoginIAM(request.Context(), domain.IAMIdentity{
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

func (s *Server) issueLoginAPIKey(response http.ResponseWriter, request *http.Request) {
	var input keyRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "PARAM_INVALID", err.Error(), "body")
		return
	}
	key, err := s.service.IssueLoginAPIKey(
		request.Context(), bearerToken(request.Header.Get("Authorization")),
		input.Name, input.Permission)
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusCreated, key)
}

func (s *Server) me(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, authFromContext(request.Context()))
}

func (s *Server) listNamespaces(response http.ResponseWriter, request *http.Request) {
	values, err := s.service.ListNamespaces(request.Context(), identityFromContext(request.Context()))
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusOK, values)
}

func (s *Server) createNamespace(response http.ResponseWriter, request *http.Request) {
	var input namespaceRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "PARAM_INVALID", err.Error(), "body")
		return
	}
	value, err := s.service.CreateNamespace(
		request.Context(), identityFromContext(request.Context()), input.DisplayName)
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusCreated, value)
}

func (s *Server) getNamespace(response http.ResponseWriter, request *http.Request) {
	value, err := s.service.GetNamespace(
		request.Context(), identityFromContext(request.Context()), request.PathValue("namespaceId"))
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Server) updateNamespace(response http.ResponseWriter, request *http.Request) {
	var input namespaceRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "PARAM_INVALID", err.Error(), "body")
		return
	}
	value, err := s.service.UpdateNamespace(
		request.Context(), identityFromContext(request.Context()),
		request.PathValue("namespaceId"), input.DisplayName)
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Server) deleteNamespace(response http.ResponseWriter, request *http.Request) {
	err := s.service.DeleteNamespace(
		request.Context(), identityFromContext(request.Context()), request.PathValue("namespaceId"))
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (s *Server) listAPIKeys(response http.ResponseWriter, request *http.Request) {
	keys, err := s.service.ListAPIKeys(
		request.Context(), identityFromContext(request.Context()), request.PathValue("namespaceId"))
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusOK, keys)
}

func (s *Server) createAPIKey(response http.ResponseWriter, request *http.Request) {
	var input keyRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "PARAM_INVALID", err.Error(), "body")
		return
	}
	key, err := s.service.CreateAPIKey(
		request.Context(), identityFromContext(request.Context()),
		request.PathValue("namespaceId"), input.Name, input.Permission)
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	writeJSON(response, http.StatusCreated, key)
}

func (s *Server) deleteAPIKey(response http.ResponseWriter, request *http.Request) {
	err := s.service.DeleteAPIKey(
		request.Context(), identityFromContext(request.Context()),
		request.PathValue("namespaceId"), request.PathValue("keyId"))
	if err != nil {
		s.handleError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (s *Server) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(response http.ResponseWriter, request *http.Request) {
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

func (s *Server) openapi(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	response.Header().Set("Cache-Control", "public, max-age=300")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(openAPISpec)
}

func (s *Server) authenticate(required string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		auth, err := s.service.Authenticate(request.Context(), request.Header.Get("X-API-Key"))
		if err != nil {
			s.handleError(response, request, err)
			return
		}
		if required == domain.PermissionWrite && auth.Permission != domain.PermissionWrite {
			writeError(response, http.StatusForbidden, "FORBIDDEN", "write permission is required", "X-API-Key")
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		ctx := context.WithValue(request.Context(), authContextKey{}, auth)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func (s *Server) requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestID, err := idgen.New("req_")
		if err != nil {
			requestID = "unavailable"
		}
		response.Header().Set("X-Request-Id", requestID)
		response.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(response, request)
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
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

func (s *Server) handleError(response http.ResponseWriter, request *http.Request, err error) {
	var applicationError *service.Error
	if !errors.As(err, &applicationError) {
		s.logger.Error("unhandled request error",
			"method", request.Method, "path", request.URL.Path, "error", err)
		writeError(response, http.StatusInternalServerError,
			"INTERNAL_ERROR", "internal server error", "")
		return
	}
	status := http.StatusInternalServerError
	switch applicationError.Kind {
	case service.KindInvalid:
		status = http.StatusBadRequest
	case service.KindUnauthorized:
		status = http.StatusUnauthorized
	case service.KindNotFound:
		status = http.StatusNotFound
	case service.KindConflict:
		status = http.StatusConflict
	case service.KindInternal:
		s.logger.Error("request failed",
			"method", request.Method, "path", request.URL.Path, "error", err)
	}
	message := applicationError.Message
	if applicationError.Kind == service.KindInternal {
		message = "internal server error"
	}
	writeError(response, status, applicationError.Code, message, applicationError.Target)
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

func authFromContext(ctx context.Context) domain.AuthContext {
	auth, _ := ctx.Value(authContextKey{}).(domain.AuthContext)
	return auth
}

func identityFromContext(ctx context.Context) domain.Identity {
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
