package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/adapters/nas/nastest"
)

func assertSpeechContains(t *testing.T, speech string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(speech, part) {
			t.Errorf("expected %q in speech:\n%s", part, speech)
		}
	}
}

func TestTVAsksDiskSpaceFromFakeNAS(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())

	speech := sentinel.ask(t, testTVToken, "ổ cứng còn bao nhiêu")

	assertSpeechContains(t, speech,
		"Ổ /srv/media: còn trống 2,0 TB trên 8,0 TB (đã dùng 75%).",
		"Ổ /srv/backup: còn trống 256,0 GB trên 4,0 TB (đã dùng 94%). Ổ này sắp đầy.",
	)
}

func TestTVAsksStoppedContainers(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())

	speech := sentinel.ask(t, testTVToken, "container nào đang dừng")

	assertSpeechContains(t, speech, "Docker: 2/3 container đang chạy. Đang dừng: radarr.")
}

func TestTVAsksFullNASStatus(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())

	speech := sentinel.ask(t, testTVToken, "trạng thái nas")

	assertSpeechContains(t, speech,
		"Máy omv-nas đang hoạt động, tải hệ thống 1 phút: 0,42.",
		"Ổ /srv/media",
		"Docker: 2/3",
	)
	if strings.Contains(speech, "hoạt động bình thường") {
		t.Errorf("hardcoded status leaked into speech:\n%s", speech)
	}
}

func TestTVReportsDockerDown(t *testing.T) {
	fake := nastest.DefaultNAS()
	fake.DockerDown = true
	sentinel := startSentinel(t, testTVToken, fake)

	speech := sentinel.ask(t, testTVToken, "docker status")

	assertSpeechContains(t, speech, "Không kết nối được Docker")
}

func TestWebDashboardReadsNASStatusJSON(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())

	response, payload := sentinel.request(
		t, nil, http.MethodGet, "/api/nas/status", testTVToken, nil,
	)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", response.StatusCode, payload)
	}

	var body struct {
		OK  bool `json:"ok"`
		NAS struct {
			Host struct {
				Hostname string `json:"hostname"`
			} `json:"host"`
			Disks []struct {
				Mount     string `json:"mount"`
				FreeBytes uint64 `json:"free_bytes"`
			} `json:"disks"`
			Containers []struct {
				Name  string `json:"name"`
				State string `json:"state"`
			} `json:"containers"`
		} `json:"nas"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("decode: %v\n%s", err, payload)
	}
	if !body.OK || body.NAS.Host.Hostname != "omv-nas" {
		t.Fatalf("unexpected body: %s", payload)
	}
	if len(body.NAS.Disks) != 2 || body.NAS.Disks[0].FreeBytes != 2*nastest.TiB {
		t.Errorf("unexpected disks: %+v", body.NAS.Disks)
	}
	if len(body.NAS.Containers) != 3 || body.NAS.Containers[2].State != "exited" {
		t.Errorf("unexpected containers: %+v", body.NAS.Containers)
	}
}
