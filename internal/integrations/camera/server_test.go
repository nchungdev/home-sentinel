package camera

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleIndex(t *testing.T) {
	server := NewCameraServer(0, 1984)
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	contentType := recorder.Header().Get("Content-Type")
	if !bytes.Contains([]byte(contentType), []byte("application/json")) {
		t.Fatalf("expected application/json, got %s", contentType)
	}

	body := recorder.Body.String()
	if !bytes.Contains([]byte(body), []byte("camera-service")) {
		t.Fatalf("expected camera-service in body")
	}
}

func TestHandleSecurityStatusAndWhitelist(t *testing.T) {
	server := NewCameraServer(0, 1984)

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	statusRecorder := httptest.NewRecorder()
	server.handleSecurityStatus(statusRecorder, statusRequest)

	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", statusRecorder.Code)
	}

	var statusResponse map[string]interface{}
	if err := json.NewDecoder(statusRecorder.Body).Decode(&statusResponse); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if statusResponse["ok"] != true {
		t.Fatalf("expected ok true")
	}

	addPayload, _ := json.Marshal(map[string]string{"ip": "192.168.1.199"})
	addRequest := httptest.NewRequest(
		http.MethodPost, "/api/security/whitelist/add", bytes.NewReader(addPayload),
	)
	addRecorder := httptest.NewRecorder()
	server.handleWhitelistAdd(addRecorder, addRequest)

	if addRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", addRecorder.Code)
	}

	server.sentinel.mutex.RLock()
	whitelisted := server.sentinel.whitelistIPs["192.168.1.199"]
	server.sentinel.mutex.RUnlock()

	if !whitelisted {
		t.Fatalf("expected 192.168.1.199 to be whitelisted")
	}

	removePayload, _ := json.Marshal(map[string]string{"ip": "192.168.1.199"})
	removeRequest := httptest.NewRequest(
		http.MethodPost, "/api/security/whitelist/remove", bytes.NewReader(removePayload),
	)
	removeRecorder := httptest.NewRecorder()
	server.handleWhitelistRemove(removeRecorder, removeRequest)

	server.sentinel.mutex.RLock()
	stillWhitelisted := server.sentinel.whitelistIPs["192.168.1.199"]
	server.sentinel.mutex.RUnlock()

	if stillWhitelisted {
		t.Fatalf("expected 192.168.1.199 to be removed")
	}
}

func TestHandleMonitorRealtime(t *testing.T) {
	server := NewCameraServer(0, 1984)
	request := httptest.NewRequest(http.MethodGet, "/api/monitor/realtime", nil)
	recorder := httptest.NewRecorder()
	server.handleMonitorRealtime(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var response map[string]interface{}
	_ = json.NewDecoder(recorder.Body).Decode(&response)
	if response["ok"] != true {
		t.Fatalf("expected ok true")
	}
	if response["sentinel"] == nil {
		t.Fatalf("expected sentinel object")
	}
}

func TestHandleTestCameraOffline(t *testing.T) {
	server := NewCameraServer(0, 1984)
	payload, _ := json.Marshal(map[string]interface{}{
		"ip":   "127.0.0.1",
		"port": 59999,
		"user": "test",
		"path": "/stream1",
	})
	request := httptest.NewRequest(
		http.MethodPost, "/api/test-camera", bytes.NewReader(payload),
	)
	recorder := httptest.NewRecorder()
	server.handleTestCamera(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var response map[string]interface{}
	_ = json.NewDecoder(recorder.Body).Decode(&response)
	if response["ok"] != false {
		t.Fatalf("expected ok false for offline camera")
	}
}

func TestHandleTtsPreview(t *testing.T) {
	server := NewCameraServer(0, 1984)
	payload, _ := json.Marshal(map[string]interface{}{
		"text":  "Alo alo",
		"voice": "Linh",
		"speed": 1.0,
	})
	request := httptest.NewRequest(
		http.MethodPost, "/api/tts/preview", bytes.NewReader(payload),
	)
	recorder := httptest.NewRecorder()
	server.handleTtsPreview(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	contentType := recorder.Header().Get("Content-Type")
	if contentType != "audio/wav" {
		t.Fatalf("expected audio/wav, got %s", contentType)
	}

	bodyBytes, _ := io.ReadAll(recorder.Body)
	if len(bodyBytes) < 44 || string(bodyBytes[:4]) != "RIFF" {
		t.Fatalf("expected valid WAV header")
	}
}

func TestHandleCameraPTZValidation(t *testing.T) {
	server := NewCameraServer(0, 1984)

	getReq := httptest.NewRequest(http.MethodGet, "/api/camera/ptz", nil)
	getRec := httptest.NewRecorder()
	server.handleCameraPTZ(getRec, getReq)
	if getRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET, got %d", getRec.Code)
	}

	emptyPayload, _ := json.Marshal(map[string]interface{}{"ip": "127.0.0.1"})
	postReq := httptest.NewRequest(
		http.MethodPost, "/api/camera/ptz", bytes.NewReader(emptyPayload),
	)
	postRec := httptest.NewRecorder()
	server.handleCameraPTZ(postRec, postReq)
	if postRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty direction, got %d", postRec.Code)
	}
}

func TestHandleCameraPrivacyValidation(t *testing.T) {
	server := NewCameraServer(0, 1984)

	getReq := httptest.NewRequest(http.MethodGet, "/api/camera/privacy", nil)
	getRec := httptest.NewRecorder()
	server.handleCameraPrivacy(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET, got %d", getRec.Code)
	}

	emptyPayload, _ := json.Marshal(map[string]interface{}{"ip": "127.0.0.1"})
	postReq := httptest.NewRequest(
		http.MethodPost, "/api/camera/privacy", bytes.NewReader(emptyPayload),
	)
	postRec := httptest.NewRecorder()
	server.handleCameraPrivacy(postRec, postReq)
	if postRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing enabled, got %d", postRec.Code)
	}
}

func TestHandleCameraEvent(t *testing.T) {
	server := NewCameraServer(0, 1984)

	invalidReq := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	invalidRec := httptest.NewRecorder()
	server.handleCameraEvent(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET, got %d", invalidRec.Code)
	}

	badPayload := bytes.NewReader([]byte(`{"zone_id": ""}`))
	badReq := httptest.NewRequest(http.MethodPost, "/api/events", badPayload)
	badRec := httptest.NewRecorder()
	server.handleCameraEvent(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing fields, got %d", badRec.Code)
	}

	validEvent := CameraEventRequest{
		ZoneID:    "sofa",
		EventType: "enter",
		Headcount: 1,
		Timestamp: 1729824000,
	}
	body, _ := json.Marshal(validEvent)
	validReq := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	validRec := httptest.NewRecorder()
	server.handleCameraEvent(validRec, validReq)
	if validRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", validRec.Code)
	}
}
