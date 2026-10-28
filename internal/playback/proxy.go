package playback

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var manifestURI = regexp.MustCompile(`URI="([^"]+)"`)

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Range")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length,Content-Range,Accept-Ranges")
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", 405)
		return
	}
	playback, endpoint, err := s.lookup(r.URL.Path)
	if err != nil {
		http.Error(w, "stream expired or invalid", 404)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, endpoint, nil)
	if err != nil {
		http.Error(w, "invalid stream", 400)
		return
	}
	for _, name := range []string{"Range", "If-Range", "If-None-Match", "If-Modified-Since"} {
		if value := r.Header.Get(name); value != "" {
			request.Header.Set(name, value)
		}
	}
	target, _ := url.Parse(endpoint)
	origin := target.Scheme + "://" + target.Host
	for name, values := range playback.headers {
		if origin == playback.trustedOrigin || name == "User-Agent" || name == "Referer" || name == "Origin" {
			request.Header[name] = values
		}
	}
	response, err := playback.client.Do(request)
	if err != nil {
		http.Error(w, "stream unavailable", 502)
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == 416 {
			w.WriteHeader(416)
			return
		}
		http.Error(w, "upstream stream unavailable", 502)
		return
	}
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	reader := bufio.NewReader(response.Body)
	prefix, _ := reader.Peek(7)
	isManifest := strings.Contains(contentType, "mpegurl") || strings.HasPrefix(string(prefix), "#EXTM3U")
	if isManifest && r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.WriteHeader(http.StatusOK)
		return
	}
	if isManifest && r.Method == http.MethodGet {
		data, err := io.ReadAll(io.LimitReader(reader, 2<<20+1))
		if err != nil || len(data) > 2<<20 {
			http.Error(w, "manifest too large", 502)
			return
		}
		manifest, err := playback.rewriteManifest(response.Request.URL, string(data))
		if err != nil {
			http.Error(w, "unsupported manifest", 502)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Content-Length", strconv.Itoa(len(manifest)))
		w.Header().Set("Cache-Control", "private, max-age=2")
		_, _ = io.WriteString(w, manifest)
		return
	}
	if !strings.HasPrefix(contentType, "video/") && !strings.HasPrefix(contentType, "audio/") &&
		!strings.Contains(contentType, "octet-stream") && !strings.Contains(contentType, "text/vtt") {
		http.Error(w, "upstream is not a supported media stream", 415)
		return
	}
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.Header().Set("Cache-Control", "private, max-age=30")
	w.WriteHeader(response.StatusCode)
	if r.Method != http.MethodHead {
		_, _ = io.CopyBuffer(w, reader, make([]byte, 64<<10))
	}
}

func (s *session) rewriteManifest(base *url.URL, manifest string) (string, error) {
	if strings.Contains(manifest, "SAMPLE-AES") || strings.Contains(manifest, "KEYFORMAT=") {
		return "", fmt.Errorf("protected_stream_unsupported")
	}
	resolve := func(value string) (string, error) {
		relative, err := url.Parse(value)
		if err != nil {
			return "", err
		}
		target := base.ResolveReference(relative)
		if target.User != nil || (target.Scheme != "http" && target.Scheme != "https") {
			return "", fmt.Errorf("invalid_stream_reference")
		}
		reference := s.reference(target.String())
		if reference == "" {
			return "", fmt.Errorf("reference_failed")
		}
		return reference, nil
	}
	lines := strings.Split(manifest, "\n")
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			reference, err := resolve(line)
			if err != nil {
				return "", err
			}
			lines[index] = reference
			continue
		}
		var rewriteError error
		lines[index] = manifestURI.ReplaceAllStringFunc(line, func(attribute string) string {
			match := manifestURI.FindStringSubmatch(attribute)
			reference, err := resolve(match[1])
			if err != nil {
				rewriteError = err
				return attribute
			}
			return `URI="` + reference + `"`
		})
		if rewriteError != nil {
			return "", rewriteError
		}
	}
	return strings.Join(lines, "\n"), nil
}
