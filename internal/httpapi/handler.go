package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/lizhicheng00/relay-controller/internal/core"
)

const (
	apiBase     = "/open-api-inner/v1/relay-controller"
	maxBodySize = 1 << 20
)

type API interface {
	CreateTunnel(context.Context, string, string, core.CreateTunnelRequest) (core.TunnelResponse, error)
	ListTunnels(context.Context, string, string, string) ([]core.TunnelListItem, error)
	GetTunnel(context.Context, string, string, string) (core.TunnelResponse, error)
	UpdateTunnel(context.Context, string, string, string, core.UpdateTunnelRequest) (bool, error)
	DeleteTunnel(context.Context, string, string, string) (bool, error)
	DeleteTunnels(context.Context, string, string) (bool, error)
	IssueTunnelToken(context.Context, string, string, string, string) (core.TunnelTokenResponse, error)
	CreatePort(context.Context, string, string, string, core.CreateTunnelPortRequest) (core.TunnelPortResponse, error)
	ListPorts(context.Context, string, string, string) ([]core.TunnelPortResponse, error)
	GetPort(context.Context, string, string, string, uint16) (core.TunnelPortResponse, error)
	UpdatePort(context.Context, string, string, string, uint16, core.UpdateTunnelPortRequest) (core.TunnelPortResponse, error)
	DeletePort(context.Context, string, string, string, uint16) (bool, error)
	GetLimits(context.Context, string, string) (core.LimitsResponse, error)
}

type Handler struct {
	api API
	log *slog.Logger
}

func New(api API, logger *slog.Logger, limiter *RateLimiter) http.Handler {
	handler := &Handler{api: api, log: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+apiBase+"/tunnels", handler.createTunnel)
	mux.HandleFunc("GET "+apiBase+"/tunnels", handler.listTunnels)
	mux.HandleFunc("DELETE "+apiBase+"/tunnels", handler.deleteTunnels)
	mux.HandleFunc("GET "+apiBase+"/tunnels/{tunnelId}", handler.getTunnel)
	mux.HandleFunc("PUT "+apiBase+"/tunnels/{tunnelId}", handler.updateTunnel)
	mux.HandleFunc("DELETE "+apiBase+"/tunnels/{tunnelId}", handler.deleteTunnel)
	mux.HandleFunc("POST "+apiBase+"/tunnels/{tunnelId}/token", handler.issueToken)
	mux.HandleFunc("POST "+apiBase+"/tunnels/{tunnelId}/ports", handler.createPort)
	mux.HandleFunc("GET "+apiBase+"/tunnels/{tunnelId}/ports", handler.listPorts)
	mux.HandleFunc("GET "+apiBase+"/tunnels/{tunnelId}/ports/{port}", handler.getPort)
	mux.HandleFunc("PUT "+apiBase+"/tunnels/{tunnelId}/ports/{port}", handler.updatePort)
	mux.HandleFunc("DELETE "+apiBase+"/tunnels/{tunnelId}/ports/{port}", handler.deletePort)
	mux.HandleFunc("GET "+apiBase+"/limits", handler.getLimits)

	var result http.Handler = mux
	if limiter != nil {
		result = limiter.Middleware(result)
	}
	return handler.recover(result)
}

func (h *Handler) createTunnel(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, err := requestContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	var body core.CreateTunnelRequest
	if err := decodeJSON(response, request, &body); err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.CreateTunnel(request.Context(), namespace, accountNamespace, body)
	h.writeResult(response, result, err)
}

func (h *Handler) listTunnels(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, err := requestContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.ListTunnels(request.Context(), namespace, accountNamespace, request.URL.Query().Get("clusterId"))
	h.writeResult(response, result, err)
}

func (h *Handler) getTunnel(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, tunnelID, err := tunnelContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.GetTunnel(request.Context(), namespace, accountNamespace, tunnelID)
	h.writeResult(response, result, err)
}

func (h *Handler) updateTunnel(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, tunnelID, err := tunnelContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	var body core.UpdateTunnelRequest
	if err := decodeJSON(response, request, &body); err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.UpdateTunnel(request.Context(), namespace, accountNamespace, tunnelID, body)
	h.writeResult(response, result, err)
}

func (h *Handler) deleteTunnel(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, tunnelID, err := tunnelContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.DeleteTunnel(request.Context(), namespace, accountNamespace, tunnelID)
	h.writeResult(response, result, err)
}

func (h *Handler) deleteTunnels(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, err := requestContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.DeleteTunnels(request.Context(), namespace, accountNamespace)
	h.writeResult(response, result, err)
}

func (h *Handler) issueToken(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, tunnelID, err := tunnelContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	scope := request.URL.Query().Get("scope")
	if scope == "" {
		h.writeError(response, &core.AppError{
			Status: http.StatusBadRequest, Code: core.CodeParamInvalid,
			Message: "required request parameter is missing", Target: "scope",
		})
		return
	}
	result, err := h.api.IssueTunnelToken(request.Context(), namespace, accountNamespace, tunnelID, scope)
	if err == nil {
		response.Header().Set("Cache-Control", "no-store")
	}
	h.writeResult(response, result, err)
}

func (h *Handler) createPort(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, tunnelID, err := tunnelContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	var body core.CreateTunnelPortRequest
	if err := decodeJSON(response, request, &body); err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.CreatePort(request.Context(), namespace, accountNamespace, tunnelID, body)
	h.writeResult(response, result, err)
}

func (h *Handler) listPorts(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, tunnelID, err := tunnelContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.ListPorts(request.Context(), namespace, accountNamespace, tunnelID)
	h.writeResult(response, result, err)
}

func (h *Handler) getPort(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, tunnelID, port, err := portContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.GetPort(request.Context(), namespace, accountNamespace, tunnelID, port)
	h.writeResult(response, result, err)
}

func (h *Handler) updatePort(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, tunnelID, port, err := portContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	var body core.UpdateTunnelPortRequest
	if err := decodeJSON(response, request, &body); err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.UpdatePort(request.Context(), namespace, accountNamespace, tunnelID, port, body)
	h.writeResult(response, result, err)
}

func (h *Handler) deletePort(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, tunnelID, port, err := portContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.DeletePort(request.Context(), namespace, accountNamespace, tunnelID, port)
	h.writeResult(response, result, err)
}

func (h *Handler) getLimits(response http.ResponseWriter, request *http.Request) {
	namespace, accountNamespace, err := requestContext(request)
	if err != nil {
		h.writeError(response, err)
		return
	}
	result, err := h.api.GetLimits(request.Context(), namespace, accountNamespace)
	h.writeResult(response, result, err)
}

func (h *Handler) writeResult(response http.ResponseWriter, result any, err error) {
	if err != nil {
		h.writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (h *Handler) writeError(response http.ResponseWriter, err error) {
	appError := core.AsAppError(err)
	if appError.Status >= http.StatusInternalServerError {
		h.log.Error("request failed", "error", appError.Error(), "stack", string(debug.Stack()))
	}
	writeJSON(response, appError.Status, appError.Response())
}

func (h *Handler) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				h.log.Error("request panic", "panic", recovered, "stack", string(debug.Stack()))
				writeJSON(response, http.StatusInternalServerError, core.Internal(nil).Response())
			}
		}()
		next.ServeHTTP(response, request)
	})
}

func requestContext(request *http.Request) (string, string, error) {
	namespace := request.Header.Get("X-Namespace")
	if strings.TrimSpace(namespace) == "" {
		return "", "", core.MissingHeader("X-Namespace")
	}
	accountNamespace := request.Header.Get("X-Account-Namespace")
	if strings.TrimSpace(accountNamespace) == "" {
		return "", "", core.MissingHeader("X-Account-Namespace")
	}
	return namespace, accountNamespace, nil
}

func tunnelContext(request *http.Request) (string, string, string, error) {
	namespace, accountNamespace, err := requestContext(request)
	if err != nil {
		return "", "", "", err
	}
	tunnelID := request.PathValue("tunnelId")
	if !core.ValidTunnelID(tunnelID) {
		return "", "", "", core.InvalidField("tunnelId", "is invalid")
	}
	return namespace, accountNamespace, tunnelID, nil
}

func portContext(request *http.Request) (string, string, string, uint16, error) {
	namespace, accountNamespace, tunnelID, err := tunnelContext(request)
	if err != nil {
		return "", "", "", 0, err
	}
	port, err := strconv.ParseUint(request.PathValue("port"), 10, 16)
	if err != nil || port == 0 {
		return "", "", "", 0, core.NewError(http.StatusBadRequest, core.CodeTunnelPortInvalid, "tunnel port invalid")
	}
	return namespace, accountNamespace, tunnelID, uint16(port), nil
}

func decodeJSON(response http.ResponseWriter, request *http.Request, target any) error {
	if contentType := request.Header.Get("Content-Type"); contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || !strings.EqualFold(mediaType, "application/json") {
			return &core.AppError{Status: http.StatusBadRequest, Code: core.CodeParamInvalid, Message: "request body is invalid", Target: "requestBody"}
		}
	}
	request.Body = http.MaxBytesReader(response, request.Body, maxBodySize)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &core.AppError{Status: http.StatusBadRequest, Code: core.CodeParamInvalid, Message: "request body is invalid", Target: "requestBody", Cause: err}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &core.AppError{Status: http.StatusBadRequest, Code: core.CodeParamInvalid, Message: "request body is invalid", Target: "requestBody", Cause: err}
	}
	return nil
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
