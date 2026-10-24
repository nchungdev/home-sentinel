package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHubIndex(t *testing.T) {
	hub := NewHubServer(0)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	hub.handleIndex(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if !bytes.Contains([]byte(body), []byte("FreeTV Home Hub")) {
		t.Fatalf("expected FreeTV Home Hub in body")
	}
}

func TestHubOverview(t *testing.T) {
	hub := NewHubServer(0)
	request := httptest.NewRequest(http.MethodGet, "/api/hub/overview", nil)
	recorder := httptest.NewRecorder()
	hub.handleOverview(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if response["ok"] != true {
		t.Fatalf("expected ok true")
	}
	devices, ok := response["devices"].([]interface{})
	if !ok || len(devices) == 0 {
		t.Fatalf("expected non-empty devices")
	}
}

func TestDeviceToggleAndSet(t *testing.T) {
	hub := NewHubServer(0)

	payload, _ := json.Marshal(map[string]string{"id": "ct_pk_1_1"})
	req := httptest.NewRequest(http.MethodPost, "/api/hub/device/toggle", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	hub.handleDeviceToggle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&response)
	dev := response["device"].(map[string]interface{})
	if dev["state"] != "on" {
		t.Fatalf("expected state on after toggle")
	}

	setPayload, _ := json.Marshal(map[string]interface{}{
		"id":          "ct_pk_2_2",
		"state":       "on",
		"temperature": 23,
	})
	setReq := httptest.NewRequest(
		http.MethodPost, "/api/hub/device/set", bytes.NewReader(setPayload),
	)
	setRec := httptest.NewRecorder()
	hub.handleDeviceSet(setRec, setReq)

	var setResponse map[string]interface{}
	_ = json.NewDecoder(setRec.Body).Decode(&setResponse)
	climateDev := setResponse["device"].(map[string]interface{})
	if climateDev["temperature"].(float64) != 23 {
		t.Fatalf("expected temperature 23")
	}
}

func TestAgentChatOrchestration(t *testing.T) {
	hub := NewHubServer(0)

	chatPayload, _ := json.Marshal(map[string]string{"text": "Bật đèn phòng khách"})
	req := httptest.NewRequest(
		http.MethodPost, "/api/hub/agent/chat", bytes.NewReader(chatPayload),
	)
	rec := httptest.NewRecorder()
	hub.handleAgentChat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&response)
	reply := response["reply"].(map[string]interface{})
	content := reply["content"].(string)
	if !bytes.Contains([]byte(content), []byte("Đã bật")) {
		t.Fatalf("expected confirmation in reply: %s", content)
	}

	scenarioPayload, _ := json.Marshal(map[string]string{"text": "Chế độ xem phim"})
	scenReq := httptest.NewRequest(
		http.MethodPost, "/api/hub/agent/chat", bytes.NewReader(scenarioPayload),
	)
	scenRec := httptest.NewRecorder()
	hub.handleAgentChat(scenRec, scenReq)

	var scenResponse map[string]interface{}
	_ = json.NewDecoder(scenRec.Body).Decode(&scenResponse)
	scenReply := scenResponse["reply"].(map[string]interface{})
	actions := scenReply["executed_actions"].([]interface{})
	if len(actions) == 0 {
		t.Fatalf("expected executed actions in movie mode")
	}

	ptzPayload, _ := json.Marshal(map[string]string{"text": "Xoay camera sang trái"})
	ptzReq := httptest.NewRequest(
		http.MethodPost, "/api/hub/agent/chat", bytes.NewReader(ptzPayload),
	)
	ptzRec := httptest.NewRecorder()
	hub.handleAgentChat(ptzRec, ptzReq)
	if ptzRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", ptzRec.Code)
	}

	ptzEmptyPayload, _ := json.Marshal(map[string]string{})
	ptzEmptyReq := httptest.NewRequest(
		http.MethodPost, "/api/hub/camera/ptz", bytes.NewReader(ptzEmptyPayload),
	)
	ptzEmptyRec := httptest.NewRecorder()
	hub.handleCameraPTZ(ptzEmptyRec, ptzEmptyReq)
	if ptzEmptyRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty direction, got %d", ptzEmptyRec.Code)
	}
}
