package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type EventHandler interface {
	HandleEvent(
		ctx context.Context,
		reqCtx *contracts.RequestContext,
		event *contracts.InputEvent,
	) (*contracts.PresentationResponse, error)
}

type EventHandlerFunc func(
	ctx context.Context,
	reqCtx *contracts.RequestContext,
	event *contracts.InputEvent,
) (*contracts.PresentationResponse, error)

func (f EventHandlerFunc) HandleEvent(
	ctx context.Context,
	reqCtx *contracts.RequestContext,
	event *contracts.InputEvent,
) (*contracts.PresentationResponse, error) {
	return f(ctx, reqCtx, event)
}

type TransportHandler struct {
	config  Config
	handler EventHandler
}

func NewHandler(config Config, handler EventHandler) *TransportHandler {
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = DefaultTimeoutDuration
	}
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = DefaultMaxBodyBytes
	}

	return &TransportHandler{
		config:  config,
		handler: handler,
	}
}

func (h *TransportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !IsIPAllowed(r.RemoteAddr, h.config.AllowedIPs) {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	if h.config.Token != "" {
		authHeader := r.Header.Get(HeaderAuthorization)
		if !VerifyBearerToken(authHeader, h.config.Token) {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
	}

	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	reqCtx, ctx, cancel := ExtractRequestContext(r, h.config.DefaultTimeout)
	defer cancel()

	event, err := ParseInputEvent(r, h.config.MaxBodyBytes)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := h.handler.HandleEvent(ctx, reqCtx, event)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) ||
			errors.Is(r.Context().Err(), context.Canceled) {
			writeJSONError(w, 499, "client disconnected")
			return
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeJSONError(w, http.StatusGatewayTimeout, "request timeout")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if response == nil {
		w.Header().Set(HeaderContentType, ContentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
		return
	}

	payload, err := response.ToJSON()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to serialize response")
		return
	}

	w.Header().Set(HeaderContentType, ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func Middleware(config Config) func(http.Handler) http.Handler {
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = DefaultTimeoutDuration
	}
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = DefaultMaxBodyBytes
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsIPAllowed(r.RemoteAddr, config.AllowedIPs) {
				writeJSONError(w, http.StatusForbidden, "forbidden")
				return
			}

			if config.Token != "" {
				authHeader := r.Header.Get(HeaderAuthorization)
				if !VerifyBearerToken(authHeader, config.Token) {
					writeJSONError(w, http.StatusUnauthorized, "unauthorized")
					return
				}
			}

			reqCtx, ctx, cancel := ExtractRequestContext(r, config.DefaultTimeout)
			defer cancel()

			ctxWithReqCtx := ContextWithRequestContext(ctx, reqCtx)
			next.ServeHTTP(w, r.WithContext(ctxWithReqCtx))
		})
	}
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set(HeaderContentType, ContentTypeJSON)
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}
