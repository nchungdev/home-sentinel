package transport

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
)

const (
	DefaultTimeoutDuration = 15 * time.Second
	DefaultMaxBodyBytes    = 64 * 1024

	HeaderAuthorization  = "Authorization"
	HeaderContentType    = "Content-Type"
	HeaderClientID       = "X-Client-ID"
	HeaderViewerProfile  = "X-Viewer-Profile"
	HeaderTimeoutMs      = "X-Timeout-Ms"
	HeaderRequestContext = "X-Request-Context"

	BearerPrefix    = "Bearer "
	ContentTypeJSON = "application/json; charset=utf-8"

	DefaultClientID = "tv-client"
)

var (
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidRequestBody = errors.New("invalid request body")
	ErrMethodNotAllowed   = errors.New("method not allowed")
)

type contextKey string

const (
	requestContextKey contextKey = "transport_request_context"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type Config struct {
	Token          string
	AllowedIPs     []string
	DefaultTimeout time.Duration
	MaxBodyBytes   int64
}

func ConfigFromEnv() Config {
	timeout := DefaultTimeoutDuration
	if rawTimeout := os.Getenv("AGENT_TIMEOUT"); rawTimeout != "" {
		if seconds, err := strconv.Atoi(rawTimeout); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	return Config{
		Token:          os.Getenv("AGENT_TOKEN"),
		AllowedIPs:     ParseAllowedIPs(os.Getenv("AGENT_ALLOW")),
		DefaultTimeout: timeout,
		MaxBodyBytes:   DefaultMaxBodyBytes,
	}
}

func ParseAllowedIPs(rawList string) []string {
	if strings.TrimSpace(rawList) == "" {
		return nil
	}

	parts := strings.Split(rawList, ",")
	allowed := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			allowed = append(allowed, trimmed)
		}
	}

	return allowed
}

func IsIPAllowed(remoteAddress string, allowedIPs []string) bool {
	if len(allowedIPs) == 0 {
		return true
	}

	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = strings.TrimSpace(remoteAddress)
	}

	clientIP := net.ParseIP(host)
	for _, allowed := range allowedIPs {
		if allowed == host {
			return true
		}

		if strings.Contains(allowed, "/") {
			_, ipNet, parseErr := net.ParseCIDR(allowed)
			if parseErr == nil && clientIP != nil && ipNet.Contains(clientIP) {
				return true
			}
			continue
		}

		parsedAllowed := net.ParseIP(allowed)
		if parsedAllowed != nil && clientIP != nil && parsedAllowed.Equal(clientIP) {
			return true
		}
	}

	return false
}

func VerifyBearerToken(authHeader string, expectedToken string) bool {
	if expectedToken == "" {
		return true
	}

	if !strings.HasPrefix(authHeader, BearerPrefix) {
		return false
	}

	actualToken := strings.TrimPrefix(authHeader, BearerPrefix)
	expectedHash := sha256.Sum256([]byte(expectedToken))
	actualHash := sha256.Sum256([]byte(actualToken))

	return subtle.ConstantTimeCompare(expectedHash[:], actualHash[:]) == 1
}

func ExtractRequestContext(
	r *http.Request,
	defaultTimeout time.Duration,
) (*contracts.RequestContext, context.Context, context.CancelFunc) {
	var requestContext *contracts.RequestContext
	if rawContext := r.Header.Get(HeaderRequestContext); rawContext != "" {
		if parsedContext, err := contracts.RequestContextFromJSON([]byte(rawContext)); err == nil {
			requestContext = parsedContext
		}
	}

	if requestContext == nil {
		requestContext = &contracts.RequestContext{}
	}

	if requestContext.ClientID == "" {
		clientID := strings.TrimSpace(r.Header.Get(HeaderClientID))
		if clientID == "" {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err == nil && host != "" {
				clientID = host
			} else if r.RemoteAddr != "" {
				clientID = r.RemoteAddr
			} else {
				clientID = DefaultClientID
			}
		}
		requestContext.ClientID = clientID
	}

	if profile := strings.TrimSpace(r.Header.Get(HeaderViewerProfile)); profile != "" {
		requestContext.ViewerProfile = &profile
	}

	requestContext.IsAuthenticated = true

	timeout := defaultTimeout
	if timeout <= 0 {
		timeout = DefaultTimeoutDuration
	}

	if rawTimeout := r.Header.Get(HeaderTimeoutMs); rawTimeout != "" {
		if ms, err := strconv.ParseInt(rawTimeout, 10, 64); err == nil && ms > 0 {
			customTimeout := time.Duration(ms) * time.Millisecond
			if customTimeout < timeout {
				timeout = customTimeout
			}
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	deadline, _ := ctx.Deadline()
	requestContext.DeadlineEpochMs = deadline.UnixMilli()

	return requestContext, ctx, cancel
}

func ParseInputEvent(r *http.Request, maxBodyBytes int64) (*contracts.InputEvent, error) {
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}

	limitedReader := http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, ErrInvalidRequestBody
	}

	event, err := contracts.InputEventFromJSON(data)
	if err != nil {
		return nil, err
	}

	if err := event.Validate(); err != nil {
		return nil, err
	}

	return event, nil
}

func ContextWithRequestContext(
	ctx context.Context,
	reqCtx *contracts.RequestContext,
) context.Context {
	return context.WithValue(ctx, requestContextKey, reqCtx)
}

func RequestContextFromContext(ctx context.Context) (*contracts.RequestContext, bool) {
	reqCtx, ok := ctx.Value(requestContextKey).(*contracts.RequestContext)
	return reqCtx, ok
}

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
