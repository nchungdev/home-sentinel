package playback

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHLSManifestAndHeadUseSessionResources(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=100000\nchild.m3u8\n"))
		}
	}))
	defer provider.Close()
	service := New()
	descriptor, err := service.Create(provider.URL+"/master.m3u8", "Fixture", "", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := httptest.NewRecorder()
		service.ServeHTTP(response, httptest.NewRequest(method, descriptor.URL, nil))
		if response.Code != http.StatusOK ||
			response.Header().Get("Content-Type") != "application/vnd.apple.mpegurl" {
			t.Fatalf("manifest %s failed: %d %s", method, response.Code, response.Body.String())
		}
		if method == http.MethodGet && (!strings.Contains(response.Body.String(), "/media/stream/") ||
			strings.Contains(response.Body.String(), "child.m3u8")) {
			t.Fatal("manifest did not rewrite child resource into opaque session reference")
		}
		if method == http.MethodHead && response.Body.Len() != 0 {
			t.Fatal("HEAD returned media content")
		}
	}
}

func TestMediaRangeIsForwardedWithoutBufferingWholeResponse(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-3" {
			t.Error("player range header lost")
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 0-3/100")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("data"))
	}))
	defer provider.Close()
	service := New()
	descriptor, err := service.Create(provider.URL+"/video.mp4", "Fixture", "", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, descriptor.URL, nil)
	request.Header.Set("Range", "bytes=0-3")
	response := httptest.NewRecorder()
	service.ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent || response.Body.String() != "data" ||
		response.Header().Get("Content-Range") != "bytes 0-3/100" {
		t.Fatalf("range response changed: %d %s", response.Code, response.Body.String())
	}
}
