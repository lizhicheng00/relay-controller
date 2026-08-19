package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"relay-controller/internal/auth"
	"relay-controller/internal/core"
)

const apiKeyHeader = "X-API-Key"

func (h *Handler) authenticate(resolver auth.Resolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		apiKey := strings.TrimSpace(request.Header.Get(apiKeyHeader))
		if apiKey == "" {
			h.writeError(response, core.MissingHeader(apiKeyHeader))
			return
		}
		principal, err := resolver.ResolveAPIKey(request.Context(), apiKey)
		if errors.Is(err, auth.ErrUnauthorized) {
			h.writeError(response, core.Unauthorized(apiKeyHeader))
			return
		}
		if err != nil {
			h.writeError(response, core.ServiceUnavailable(err))
			return
		}
		if principal.Scope != "devbridge" {
			h.writeError(response, core.Forbidden(apiKeyHeader, "API key scope is not allowed"))
			return
		}
		if !core.ValidIdentifier(principal.Namespace) || !core.ValidIdentifier(principal.AccountNamespace) {
			h.writeError(response, core.ServiceUnavailable(errors.New("management service returned an invalid identity")))
			return
		}
		next.ServeHTTP(response, request.WithContext(auth.WithPrincipal(request.Context(), principal)))
	})
}
