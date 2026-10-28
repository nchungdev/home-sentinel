package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/adapters/nas/nastest"
	"github.com/nchungdev/home-sentinel/internal/tools"
)

type fakeHomeAssistant struct {
	mutex sync.Mutex
	calls []string
	state string
}

func (f *fakeHomeAssistant) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	light := map[string]any{"entity_id": "light.phong_khach", "state": f.state,
		"attributes": map[string]any{"friendly_name": "Đèn phòng khách"}}
	switch {
	case r.URL.Path == "/api/states":
		_ = json.NewEncoder(w).Encode([]any{light, map[string]any{
			"entity_id": "sensor.nhiet_do", "state": "30", "attributes": map[string]any{}}})
	case r.URL.Path == "/api/states/light.phong_khach":
		_ = json.NewEncoder(w).Encode(light)
	case strings.HasPrefix(r.URL.Path, "/api/services/"):
		f.calls = append(f.calls, r.URL.Path)
		f.state = "on"
		_ = json.NewEncoder(w).Encode([]any{})
	default:
		http.NotFound(w, r)
	}
}

func agentToolsFor(t *testing.T) (*tools.Registry, *fakeHomeAssistant) {
	t.Helper()
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())
	homeAssistant := &fakeHomeAssistant{state: "off"}
	haServer := httptest.NewServer(homeAssistant)
	t.Cleanup(haServer.Close)
	sentinel.server.HAClient.UpdateConfig(haServer.URL, "ha-token")
	return newAgentTools(sentinel.server), homeAssistant
}

func runTool(t *testing.T, registry *tools.Registry, name, args string) (string, error) {
	t.Helper()
	return registry.Call(context.Background(), "tv", name, json.RawMessage(args))
}

func TestAgentToolCatalog(t *testing.T) {
	registry, _ := agentToolsFor(t)
	confirm := map[string]bool{"camera_ptz": true, "camera_speak": true}
	want := []string{"camera_ptz", "camera_speak", "ha_control", "ha_list_entities",
		"nas_status", "plex_recent", "plex_search", "tv_notify", "tv_open_plex", "tv_open_youtube"}

	var names []string
	for _, tool := range registry.List() {
		names = append(names, tool.Name)
		if tool.RequiresConfirmation != confirm[tool.Name] {
			t.Errorf("%s: requires confirmation = %v", tool.Name, tool.RequiresConfirmation)
		}
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("tools = %v", names)
	}
}

func TestNASStatusTool(t *testing.T) {
	registry, _ := agentToolsFor(t)

	reply, err := runTool(t, registry, "nas_status", `{"target":"disk"}`)

	if err != nil || !strings.Contains(reply, "Ổ /srv/media: còn trống 2,0 TB") {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
}

func TestHomeAssistantTools(t *testing.T) {
	registry, homeAssistant := agentToolsFor(t)

	list, err := runTool(t, registry, "ha_list_entities", `{"search":"phòng khách"}`)
	if err != nil || !strings.Contains(list, "light.phong_khach") || strings.Contains(list, "sensor") {
		t.Fatalf("list=%q err=%v", list, err)
	}

	reply, err := runTool(t, registry, "ha_control",
		`{"entity_id":"light.phong_khach","service":"turn_on"}`)
	if err != nil || !strings.Contains(reply, "Trạng thái đọc lại: on") {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
	if len(homeAssistant.calls) != 1 || homeAssistant.calls[0] != "/api/services/light/turn_on" {
		t.Fatalf("service calls = %v", homeAssistant.calls)
	}
}

func TestTVToolsReportMissingTV(t *testing.T) {
	registry, _ := agentToolsFor(t)

	_, err := runTool(t, registry, "tv_open_youtube", `{"query":"Dune trailer"}`)

	if err == nil || !strings.Contains(err.Error(), "không có TV") {
		t.Fatalf("expected missing TV error, got %v", err)
	}
}

func TestCameraToolsAskBeforeActing(t *testing.T) {
	registry, _ := agentToolsFor(t)

	reply, err := runTool(t, registry, "camera_speak", `{"text":"Xin chào"}`)

	if err != nil || !strings.Contains(reply, "CẦN XÁC NHẬN: phát qua loa camera") {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
}
