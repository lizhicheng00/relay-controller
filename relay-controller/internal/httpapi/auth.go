package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"relay-controller/internal/auth"
	"relay-controller/internal/core"
)

const (
	apiKeyHeader               = "X-API-Key"
	trustedIdentityTokenHeader = "X-Trusted-Identity-Token"
	domainIDHeader             = "X-Domain-Id"
	userIDHeader               = "X-User-Id"
)

func (h *Handler) authenticate(
	resolver auth.Resolver,
	trustedIdentityToken string,
	next http.Handler,
) http.Handler {
	trustedIdentityEnabled := trustedIdentityToken != ""
	trustedIdentityTokenHash := sha256.Sum256([]byte(trustedIdentityToken))
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		apiKey := strings.TrimSpace(request.Header.Get(apiKeyHeader))
		identityToken := request.Header.Get(trustedIdentityTokenHeader)
		if apiKey != "" && identityToken != "" {
			h.writeError(response, core.Unauthorized("authentication"))
			return
		}

		var principal auth.Principal
		var err error
		target := apiKeyHeader
		switch {
		case apiKey != "":
			principal, err = resolver.ResolveAPIKey(request.Context(), apiKey)
		case identityToken != "":
			target = trustedIdentityTokenHeader
			identityTokenHash := sha256.Sum256([]byte(identityToken))
			if !trustedIdentityEnabled ||
				subtle.ConstantTimeCompare(identityTokenHash[:], trustedIdentityTokenHash[:]) != 1 {
				h.writeError(response, core.Unauthorized(target))
				return
			}
			domainID := strings.TrimSpace(request.Header.Get(domainIDHeader))
			userID := strings.TrimSpace(request.Header.Get(userIDHeader))
			if domainID == "" {
				h.writeError(response, core.MissingHeader(domainIDHeader))
				return
			}
			if userID == "" {
				h.writeError(response, core.MissingHeader(userIDHeader))
				return
			}
			principal, err = resolver.ResolveIdentity(request.Context(), domainID, userID)
		default:
			h.writeError(response, core.MissingHeader(apiKeyHeader))
			return
		}

		if errors.Is(err, auth.ErrUnauthorized) {
			h.writeError(response, core.Unauthorized(target))
			return
		}
		if err != nil {
			h.writeError(response, core.ServiceUnavailable(err))
			return
		}
		if principal.Scope != "devbridge" {
			h.writeError(response, core.Forbidden(target, "credential scope is not allowed"))
			return
		}
		if !core.ValidIdentifier(principal.Namespace) || !core.ValidIdentifier(principal.AccountNamespace) {
			h.writeError(response, core.ServiceUnavailable(errors.New("management service returned an invalid identity")))
			return
		}
		next.ServeHTTP(response, request.WithContext(auth.WithPrincipal(request.Context(), principal)))
	})
}
