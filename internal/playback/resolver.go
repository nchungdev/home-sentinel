package playback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

type Descriptor struct {
	URL              string    `json:"url"`
	MIMEType         string    `json:"mime_type"`
	Title            string    `json:"title"`
	PreferredHandler string    `json:"preferred_handler"`
	PreferredPlayers []string  `json:"preferred_players"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type Resolver struct {
	extractor string
	hosts     map[string]bool
}

func NewResolver() *Resolver {
	extractor, _ := exec.LookPath("yt-dlp")
	hosts := map[string]bool{"youtube.com": true, "youtu.be": true, "vimeo.com": true, "archive.org": true}
	for _, host := range strings.Split(os.Getenv("MEDIA_STREAM_HOSTS"), ",") {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			hosts[host] = true
		}
	}
	return &Resolver{extractor: extractor, hosts: hosts}
}

func streamMIME(endpoint string) string {
	parsed, _ := url.Parse(endpoint)
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".mp4":
		return "video/mp4"
	default:
		return "video/*"
	}
}

func (r *Resolver) CanResolve(endpoint string) bool {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".mp4", ".m3u8", ".webm", ".mkv":
		return true
	}
	return r.extractor != "" && r.allowedHost(parsed.Hostname())
}

func (r *Resolver) allowedHost(host string) bool {
	host = strings.ToLower(host)
	for candidate := range r.hosts {
		if host == candidate || strings.HasSuffix(host, "."+candidate) {
			return true
		}
	}
	return false
}

func (r *Resolver) Resolve(ctx context.Context, endpoint string) (string, http.Header, error) {
	if !r.CanResolve(endpoint) {
		return "", nil, fmt.Errorf("trang chưa hỗ trợ trích nguồn phát")
	}
	parsed, _ := url.Parse(endpoint)
	if err := validatePublicHost(ctx, parsed.Hostname()); err != nil {
		return "", nil, err
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".mp4", ".m3u8", ".webm", ".mkv":
		return endpoint, nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, r.extractor,
		"--ignore-config", "--dump-single-json", "--skip-download", "--no-playlist",
		"--no-warnings", "--socket-timeout", "8", "--no-cache-dir",
		"--format", "best[protocol^=m3u8]/best[vcodec!=none][acodec!=none]", "--", endpoint)
	var output limitedOutput
	command.Stdout = &output
	if err := command.Run(); err != nil {
		return "", nil, fmt.Errorf("không lấy được nguồn video từ trang")
	}
	var result struct {
		URL     string            `json:"url"`
		HasDRM  bool              `json:"has_drm"`
		Headers map[string]string `json:"http_headers"`
	}
	if json.Unmarshal(output.Bytes(), &result) != nil || result.URL == "" || result.HasDRM {
		return "", nil, fmt.Errorf("nguồn không có video phát trực tiếp được hỗ trợ")
	}
	target, err := url.Parse(result.URL)
	if err != nil || target.User != nil || (target.Scheme != "http" && target.Scheme != "https") {
		return "", nil, fmt.Errorf("invalid_stream_url")
	}
	if err := validatePublicHost(ctx, target.Hostname()); err != nil {
		return "", nil, err
	}
	headers := http.Header{}
	for _, name := range []string{"User-Agent", "Referer", "Origin"} {
		if value := result.Headers[name]; value != "" {
			headers.Set(name, value)
		}
	}
	return result.URL, headers, nil
}

type limitedOutput struct{ bytes.Buffer }

func (b *limitedOutput) Write(data []byte) (int, error) {
	if b.Len()+len(data) > 2<<20 {
		return 0, fmt.Errorf("extractor_output_too_large")
	}
	return b.Buffer.Write(data)
}
