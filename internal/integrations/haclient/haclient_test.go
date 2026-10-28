package haclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleCommand(t *testing.T) {
	mockStates := []EntityState{
		{
			EntityID:   "light.living_room",
			State:      "off",
			Attributes: map[string]interface{}{"friendly_name": "Đèn phòng khách"},
		},
		{
			EntityID:   "fan.bedroom",
			State:      "on",
			Attributes: map[string]interface{}{"friendly_name": "Quạt phòng ngủ"},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/states" {
			_ = json.NewEncoder(w).Encode(mockStates)
			return
		}
		if r.URL.Path == "/api/services/light/turn_on" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/states/light.living_room" {
			_ = json.NewEncoder(w).Encode(EntityState{
				EntityID: "light.living_room",
				State:    "on",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token")

	reply, handled := client.HandleCommand("bật đèn phòng khách")
	if !handled {
		t.Fatalf("expected handled true")
	}
	if reply == "" {
		t.Fatalf("expected reply not empty")
	}

	_, handledNotCmd := client.HandleCommand("thời tiết hôm nay")
	if handledNotCmd {
		t.Fatalf("expected handled false for non-command")
	}
}
