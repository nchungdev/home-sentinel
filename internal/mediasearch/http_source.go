package mediasearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

const resultLimit = 8

type httpSource struct {
	id        string
	name      string
	kind      string
	baseURL   string
	publicURL string
	apiKey    string
	keyHeader string
	probePath string
	client    *http.Client
}

func (s *httpSource) do(ctx context.Context, method, path string, body, result any) error {
	base, err := url.Parse(strings.TrimRight(s.baseURL, "/") + "/")
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return ErrNotConfigured
	}
	relative, err := url.Parse(strings.TrimPrefix(path, "/"))
	if err != nil {
		return ErrNotConfigured
	}
	endpoint := base.ResolveReference(relative)
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return ErrNotConfigured
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if s.keyHeader != "" && s.apiKey != "" {
		request.Header.Set(s.keyHeader, s.apiKey)
		if s.id == "jellyfin" {
			request.Header.Set("Authorization", `MediaBrowser Token="`+s.apiKey+`"`)
		}
	}
	response, err := s.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("source_unreachable")
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return ErrAuthRequired
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("http_%d", response.StatusCode)
	}
	if result == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(result); err != nil {
		return fmt.Errorf("invalid_source_response")
	}
	return nil
}

func (s *httpSource) Capability(ctx context.Context) contracts.MediaSourceCapability {
	capability := contracts.MediaSourceCapability{ID: s.id, Name: s.name, Kind: s.kind,
		Status: "unreachable", Actions: []string{}}
	if s.baseURL == "" {
		capability.Status = "not_configured"
		return capability
	}
	var info map[string]any
	err := s.do(ctx, http.MethodGet, s.probePath, nil, &info)
	if err != nil {
		if errors.Is(err, ErrAuthRequired) {
			capability.Detected, capability.Status = true, "auth_required"
		}
		return capability
	}
	if !validProbe(s.id, info) {
		capability.Status = "invalid_service"
		return capability
	}
	capability.Detected = true
	if s.keyHeader != "" && s.apiKey == "" {
		capability.Status = "auth_required"
		return capability
	}
	capability.Available, capability.Status = true, "ready"
	capability.Actions = []string{"open_url"}
	return capability
}

func (s *httpSource) CacheIdentity() string {
	tokenDigest := sha256.Sum256([]byte(s.apiKey))
	return s.id + ":" + strings.TrimRight(s.baseURL, "/") + ":" + hex.EncodeToString(tokenDigest[:])
}

func (s *httpSource) link(path string) string {
	base := s.publicURL
	if base == "" {
		base = s.baseURL
	}
	return strings.TrimRight(base, "/") + path
}

func linkAction(label, endpoint string) contracts.MediaAction {
	return contracts.MediaAction{Type: "open_url", Label: label, URL: endpoint, Enabled: true}
}

func playAction(endpoint string) contracts.MediaAction {
	return contracts.MediaAction{Type: "play", Label: "Phát trực tiếp", URL: endpoint, Enabled: true}
}
