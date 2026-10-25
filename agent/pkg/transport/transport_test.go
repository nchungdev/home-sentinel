package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
)

const (
	testSecretToken = "test-token-secret-12345"
)

func createTestInputEventJSON() []byte {
	event := contracts.InputEvent{
		RequestID: "req-101",
		SessionID: "sess-202",
		Text:      "mở đèn phòng khách",
		Locale:    "vi-VN",
		InputType: "voice",
	}

	payload, _ := event.ToJSON()
	return payload
}

func TestMissingTokenReturns401(t *testing.T) {
	cfg := Config{
		Token: testSecretToken,
	}

	handler := NewHandler(cfg, EventHandlerFunc(
		func(
			ctx context.Context,
			reqCtx *contracts.RequestContext,
			event *contracts.InputEvent,
		) (*contracts.PresentationResponse, error) {
			return &contracts.PresentationResponse{
				SpeechText: "ok",
			}, nil
		},
	))

	server := httptest.NewServer(handler)
	defer server.Close()

	req, err := http.NewRequest(
		http.MethodPost,
		server.URL,
		bytes.NewReader(createTestInputEventJSON()),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if errResp.Error != "unauthorized" {
		t.Fatalf("expected error 'unauthorized', got %q", errResp.Error)
	}
}

func TestInvalidTokenReturns401(t *testing.T) {
	cfg := Config{
		Token: testSecretToken,
	}

	handler := NewHandler(cfg, EventHandlerFunc(
		func(
			ctx context.Context,
			reqCtx *contracts.RequestContext,
			event *contracts.InputEvent,
		) (*contracts.PresentationResponse, error) {
			return &contracts.PresentationResponse{
				SpeechText: "ok",
			}, nil
		},
	))

	server := httptest.NewServer(handler)
	defer server.Close()

	invalidHeaders := []string{
		"Bearer wrong-token-value",
		"Basic dXNlcjpwYXNz",
		"Bearer ",
		"Token something",
	}

	for _, authHeader := range invalidHeaders {
		req, err := http.NewRequest(
			http.MethodPost,
			server.URL,
			bytes.NewReader(createTestInputEventJSON()),
		)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set(HeaderAuthorization, authHeader)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		if resp.StatusCode != http.StatusUnauthorized {
			resp.Body.Close()
			t.Fatalf("for header %q expected 401, got %d", authHeader, resp.StatusCode)
		}

		var errResp ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		resp.Body.Close()

		if errResp.Error != "unauthorized" {
			t.Fatalf("expected error 'unauthorized', got %q", errResp.Error)
		}
	}
}

func TestValidTokenReturns200(t *testing.T) {
	cfg := Config{
		Token: testSecretToken,
	}

	invoked := false
	handler := NewHandler(cfg, EventHandlerFunc(
		func(
			ctx context.Context,
			reqCtx *contracts.RequestContext,
			event *contracts.InputEvent,
		) (*contracts.PresentationResponse, error) {
			invoked = true
			return &contracts.PresentationResponse{
				SpeechText: "Đã thực hiện xong",
			}, nil
		},
	))

	server := httptest.NewServer(handler)
	defer server.Close()

	req, err := http.NewRequest(
		http.MethodPost,
		server.URL,
		bytes.NewReader(createTestInputEventJSON()),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set(HeaderAuthorization, "Bearer "+testSecretToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if !invoked {
		t.Fatal("expected handler to be invoked")
	}

	var presResp contracts.PresentationResponse
	if err := json.NewDecoder(resp.Body).Decode(&presResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if presResp.SpeechText != "Đã thực hiện xong" {
		t.Fatalf("unexpected speech text: %q", presResp.SpeechText)
	}
}

func TestIPWhitelistEnforcement(t *testing.T) {
	cfg := Config{
		Token:      testSecretToken,
		AllowedIPs: []string{"192.168.1.50", "10.0.0.0/24"},
	}

	handler := NewHandler(cfg, EventHandlerFunc(
		func(
			ctx context.Context,
			reqCtx *contracts.RequestContext,
			event *contracts.InputEvent,
		) (*contracts.PresentationResponse, error) {
			return &contracts.PresentationResponse{SpeechText: "allowed"}, nil
		},
	))

	testCases := []struct {
		remoteAddr     string
		expectedStatus int
	}{
		{"192.168.1.50:4321", http.StatusOK},
		{"10.0.0.42:9999", http.StatusOK},
		{"192.168.1.51:4321", http.StatusForbidden},
		{"127.0.0.1:8788", http.StatusForbidden},
		{"10.0.1.1:5555", http.StatusForbidden},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(
			http.MethodPost,
			"/v2/ask",
			bytes.NewReader(createTestInputEventJSON()),
		)
		req.RemoteAddr = tc.remoteAddr
		req.Header.Set(HeaderAuthorization, "Bearer "+testSecretToken)

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		if recorder.Code != tc.expectedStatus {
			t.Fatalf("for remote addr %s expected %d, got %d",
				tc.remoteAddr, tc.expectedStatus, recorder.Code)
		}

		if tc.expectedStatus == http.StatusForbidden {
			var errResp ErrorResponse
			if err := json.NewDecoder(recorder.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode forbidden response: %v", err)
			}
			if errResp.Error != "forbidden" {
				t.Fatalf("expected error 'forbidden', got %q", errResp.Error)
			}
		}
	}
}

func TestEmptyIPWhitelistAllowsAny(t *testing.T) {
	cfg := Config{
		Token:      testSecretToken,
		AllowedIPs: nil,
	}

	handler := NewHandler(cfg, EventHandlerFunc(
		func(
			ctx context.Context,
			reqCtx *contracts.RequestContext,
			event *contracts.InputEvent,
		) (*contracts.PresentationResponse, error) {
			return &contracts.PresentationResponse{SpeechText: "allowed"}, nil
		},
	))

	remoteAddresses := []string{
		"192.168.1.100:1111",
		"10.20.30.40:2222",
		"127.0.0.1:3333",
	}

	for _, remote := range remoteAddresses {
		req := httptest.NewRequest(
			http.MethodPost,
			"/v2/ask",
			bytes.NewReader(createTestInputEventJSON()),
		)
		req.RemoteAddr = remote
		req.Header.Set(HeaderAuthorization, "Bearer "+testSecretToken)

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200 for remote addr %s, got %d", remote, recorder.Code)
		}
	}
}

func TestParseInputEventAndRequestContext(t *testing.T) {
	cfg := Config{
		Token:          testSecretToken,
		DefaultTimeout: 20 * time.Second,
	}

	var capturedReqCtx *contracts.RequestContext
	var capturedEvent *contracts.InputEvent

	handler := NewHandler(cfg, EventHandlerFunc(
		func(
			ctx context.Context,
			reqCtx *contracts.RequestContext,
			event *contracts.InputEvent,
		) (*contracts.PresentationResponse, error) {
			capturedReqCtx = reqCtx
			capturedEvent = event
			return &contracts.PresentationResponse{SpeechText: "verified"}, nil
		},
	))

	req := httptest.NewRequest(
		http.MethodPost,
		"/v2/ask",
		bytes.NewReader(createTestInputEventJSON()),
	)
	req.RemoteAddr = "192.168.1.77:5000"
	req.Header.Set(HeaderAuthorization, "Bearer "+testSecretToken)
	req.Header.Set(HeaderClientID, "custom-tv-unit-1")
	req.Header.Set(HeaderViewerProfile, "kids-mode")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	if capturedEvent == nil || capturedEvent.RequestID != "req-101" {
		t.Fatalf("expected event RequestID 'req-101', got %+v", capturedEvent)
	}
	if capturedEvent.Text != "mở đèn phòng khách" {
		t.Fatalf("expected event Text 'mở đèn phòng khách', got %q", capturedEvent.Text)
	}

	if capturedReqCtx == nil {
		t.Fatal("expected RequestContext to be passed to handler")
	}
	if capturedReqCtx.ClientID != "custom-tv-unit-1" {
		t.Fatalf("expected ClientID 'custom-tv-unit-1', got %q", capturedReqCtx.ClientID)
	}
	if !capturedReqCtx.IsAuthenticated {
		t.Fatal("expected IsAuthenticated to be true")
	}
	if capturedReqCtx.ViewerProfile == nil || *capturedReqCtx.ViewerProfile != "kids-mode" {
		t.Fatalf("expected ViewerProfile 'kids-mode', got %+v", capturedReqCtx.ViewerProfile)
	}
	if capturedReqCtx.DeadlineEpochMs <= time.Now().UnixMilli() {
		t.Fatalf("expected future DeadlineEpochMs, got %d", capturedReqCtx.DeadlineEpochMs)
	}
}

func TestInvalidInputEventReturns400(t *testing.T) {
	cfg := Config{
		Token: testSecretToken,
	}

	handler := NewHandler(cfg, EventHandlerFunc(
		func(
			ctx context.Context,
			reqCtx *contracts.RequestContext,
			event *contracts.InputEvent,
		) (*contracts.PresentationResponse, error) {
			return &contracts.PresentationResponse{SpeechText: "ok"}, nil
		},
	))

	invalidBodies := [][]byte{
		[]byte("not-json"),
		[]byte(`{"request_id": ""}`),
		[]byte(`{"request_id": "req-1", "text": ""}`),
		[]byte(""),
	}

	for _, body := range invalidBodies {
		req := httptest.NewRequest(http.MethodPost, "/v2/ask", bytes.NewReader(body))
		req.Header.Set(HeaderAuthorization, "Bearer "+testSecretToken)

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for body %s, got %d", string(body), recorder.Code)
		}
	}
}

func TestMethodNotAllowedForGet(t *testing.T) {
	cfg := Config{
		Token: testSecretToken,
	}

	handler := NewHandler(cfg, EventHandlerFunc(
		func(
			ctx context.Context,
			reqCtx *contracts.RequestContext,
			event *contracts.InputEvent,
		) (*contracts.PresentationResponse, error) {
			return &contracts.PresentationResponse{SpeechText: "ok"}, nil
		},
	))

	req := httptest.NewRequest(http.MethodGet, "/v2/ask", nil)
	req.Header.Set(HeaderAuthorization, "Bearer "+testSecretToken)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", recorder.Code)
	}
}

func TestClientDisconnectContextCancelled(t *testing.T) {
	cfg := Config{
		Token:          testSecretToken,
		DefaultTimeout: 5 * time.Second,
	}

	cancelledSignal := make(chan struct{})

	handler := NewHandler(cfg, EventHandlerFunc(
		func(
			ctx context.Context,
			reqCtx *contracts.RequestContext,
			event *contracts.InputEvent,
		) (*contracts.PresentationResponse, error) {
			select {
			case <-ctx.Done():
				close(cancelledSignal)
				return nil, ctx.Err()
			case <-time.After(3 * time.Second):
				return &contracts.PresentationResponse{SpeechText: "done"}, nil
			}
		},
	))

	req := httptest.NewRequest(
		http.MethodPost,
		"/v2/ask",
		bytes.NewReader(createTestInputEventJSON()),
	)
	req.Header.Set(HeaderAuthorization, "Bearer "+testSecretToken)

	requestContext, cancel := context.WithCancel(req.Context())
	req = req.WithContext(requestContext)

	recorder := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(recorder, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-cancelledSignal:
	case <-time.After(1 * time.Second):
		t.Fatal("handler did not receive context cancellation")
	}

	<-done

	if recorder.Code != 499 {
		t.Fatalf("expected status 499 on disconnect, got %d", recorder.Code)
	}
}

func TestMiddlewareRequestContextInjection(t *testing.T) {
	cfg := Config{
		Token:      testSecretToken,
		AllowedIPs: []string{"192.168.1.0/24"},
	}

	middleware := Middleware(cfg)

	var extractedContext *contracts.RequestContext
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCtx, ok := RequestContextFromContext(r.Context())
		if ok {
			extractedContext = reqCtx
		}
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(innerHandler)

	req := httptest.NewRequest(http.MethodPost, "/v2/ask", nil)
	req.RemoteAddr = "192.168.1.99:54321"
	req.Header.Set(HeaderAuthorization, "Bearer "+testSecretToken)
	req.Header.Set(HeaderClientID, "living-room-tv")

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from middleware, got %d", recorder.Code)
	}

	if extractedContext == nil {
		t.Fatal("expected request context to be injected into context")
	}
	if extractedContext.ClientID != "living-room-tv" {
		t.Fatalf("expected ClientID 'living-room-tv', got %q", extractedContext.ClientID)
	}
	if !extractedContext.IsAuthenticated {
		t.Fatal("expected IsAuthenticated to be true")
	}
}

func TestConfigFromEnvAndParsing(t *testing.T) {
	os.Setenv("AGENT_TOKEN", "env-secret-token")
	os.Setenv("AGENT_ALLOW", "192.168.1.1, 10.0.0.1/24, 127.0.0.1")
	os.Setenv("AGENT_TIMEOUT", "30")
	defer func() {
		os.Unsetenv("AGENT_TOKEN")
		os.Unsetenv("AGENT_ALLOW")
		os.Unsetenv("AGENT_TIMEOUT")
	}()

	cfg := ConfigFromEnv()

	if cfg.Token != "env-secret-token" {
		t.Fatalf("unexpected token: %q", cfg.Token)
	}
	if len(cfg.AllowedIPs) != 3 {
		t.Fatalf("expected 3 allowed IPs, got %d", len(cfg.AllowedIPs))
	}
	if cfg.AllowedIPs[0] != "192.168.1.1" || cfg.AllowedIPs[1] != "10.0.0.1/24" {
		t.Fatalf("unexpected allowed IPs: %+v", cfg.AllowedIPs)
	}
	if cfg.DefaultTimeout != 30*time.Second {
		t.Fatalf("expected timeout 30s, got %v", cfg.DefaultTimeout)
	}
}

func TestHandlerInternalError(t *testing.T) {
	cfg := Config{
		Token: testSecretToken,
	}

	handler := NewHandler(cfg, EventHandlerFunc(
		func(
			ctx context.Context,
			reqCtx *contracts.RequestContext,
			event *contracts.InputEvent,
		) (*contracts.PresentationResponse, error) {
			return nil, errors.New("upstream failure")
		},
	))

	req := httptest.NewRequest(
		http.MethodPost,
		"/v2/ask",
		bytes.NewReader(createTestInputEventJSON()),
	)
	req.Header.Set(HeaderAuthorization, "Bearer "+testSecretToken)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}

	var errResp ErrorResponse
	_ = json.NewDecoder(recorder.Body).Decode(&errResp)
	if errResp.Error != "upstream failure" {
		t.Fatalf("expected error 'upstream failure', got %q", errResp.Error)
	}
}
