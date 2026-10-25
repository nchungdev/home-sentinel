package camera

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (server *CameraServer) handleSecurityStatus(
	writer http.ResponseWriter,
	request *http.Request,
) {
	status := server.sentinel.GetStatus()
	server.writeJSON(writer, map[string]interface{}{"ok": true, "status": status}, 200)
}

func (server *CameraServer) handleSecurityConfig(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload SecurityConfigRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	status := server.sentinel.GetStatus()
	speaker := status.AlarmSpeakerEnabled
	tv := status.AlarmTvEnabled
	if payload.AlarmSpeaker != nil {
		speaker = *payload.AlarmSpeaker
	}
	if payload.AlarmTv != nil {
		tv = *payload.AlarmTv
	}
	server.sentinel.SetAlarms(speaker, tv)

	server.writeJSON(
		writer,
		map[string]interface{}{"ok": true, "status": server.sentinel.GetStatus()},
		200,
	)
}

func (server *CameraServer) handleWhitelistAdd(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload WhitelistIPRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)
	server.sentinel.AddWhitelistIP(payload.IP)
	server.writeJSON(
		writer,
		map[string]interface{}{"ok": true, "status": server.sentinel.GetStatus()},
		200,
	)
}

func (server *CameraServer) handleWhitelistRemove(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload WhitelistIPRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)
	server.sentinel.RemoveWhitelistIP(payload.IP)
	server.writeJSON(
		writer,
		map[string]interface{}{"ok": true, "status": server.sentinel.GetStatus()},
		200,
	)
}

func (server *CameraServer) handleSecurityTestAlarm(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload SpeakRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	cameraHost := payload.IP
	if cameraHost == "" {
		cameraHost = server.defaultCameraIP
	}
	effectivePassword := payload.CloudPassword
	if effectivePassword == "" {
		effectivePassword = payload.Password
	}

	speakerCallback := func(wavData []byte) error {
		return BroadcastAudio(
			cameraHost, payload.User, effectivePassword, wavData,
		)
	}

	server.sentinel.TriggerSecurityAlert("192.168.1.99 (Test)", speakerCallback)
	server.writeJSON(
		writer,
		map[string]interface{}{
			"ok":      true,
			"message": "Đã kích hoạt thử nghiệm báo động Sentinel (Loa + TV)",
		},
		200,
	)
}

func (server *CameraServer) handleCameraEvent(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodPost {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Chỉ hỗ trợ POST"},
			http.StatusMethodNotAllowed,
		)
		return
	}

	var payload CameraEventRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Dữ liệu JSON không hợp lệ"},
			http.StatusBadRequest,
		)
		return
	}

	if payload.ZoneID == "" || payload.EventType == "" {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Thiếu zone_id hoặc event_type"},
			http.StatusBadRequest,
		)
		return
	}

	if payload.EventType == "enter" && server.sentinel.alarmTvEnabled {
		alertMsg := fmt.Sprintf("Phát hiện người tại khu vực %s", payload.ZoneID)
		if payload.ZoneID == "sofa" {
			alertMsg = "Chào mừng bạn! Đã phát hiện người ngồi sofa."
		}
		server.sentinel.sendTvUdpNotification("Tương tác Camera", alertMsg)
	}

	server.writeJSON(
		writer,
		map[string]interface{}{
			"ok":      true,
			"message": "Đã ghi nhận sự kiện tương tác camera thành công",
		},
		200,
	)
}

func (server *CameraServer) handleMonitorRealtime(
	writer http.ResponseWriter,
	request *http.Request,
) {
	status := server.sentinel.GetStatus()
	server.writeJSON(
		writer,
		map[string]interface{}{
			"ok":              true,
			"sentinel":        status,
			"go2rtcPort":      server.go2rtcPort,
			"defaultCameraIp": server.defaultCameraIP,
		},
		200,
	)
}
