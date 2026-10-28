package camera

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (server *CameraServer) handleCameraPTZ(
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

	var payload CameraPTZRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	cameraHost := payload.IP
	if cameraHost == "" {
		cameraHost = server.defaultCameraIP
	}

	effectivePassword := payload.CloudPassword
	if effectivePassword == "" {
		effectivePassword = payload.Password
	}
	if effectivePassword == "" {
		server.writeJSON(
			writer,
			map[string]interface{}{
				"ok":    false,
				"error": "Thiếu mật khẩu Camera (hãy nhập vào Cài Đặt Camera)",
			},
			http.StatusBadRequest,
		)
		return
	}

	session := GetTapoSession(cameraHost, payload.User, effectivePassword)

	var moveErr error
	if payload.Direction != "" {
		moveErr = session.MoveDirection(payload.Direction, payload.Step)
	} else if payload.X != 0 || payload.Y != 0 {
		moveErr = session.MoveMotor(payload.X, payload.Y)
	} else {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Thiếu hướng hoặc tọa độ xoay"},
			http.StatusBadRequest,
		)
		return
	}

	if moveErr != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{
				"ok":    false,
				"error": fmt.Sprintf("Lỗi điều khiển xoay camera: %v", moveErr),
			},
			500,
		)
		return
	}

	server.writeJSON(
		writer,
		map[string]interface{}{
			"ok":        true,
			"message":   fmt.Sprintf("Đã điều khiển xoay camera thành công (%s)", payload.Direction),
			"direction": payload.Direction,
		},
		200,
	)
}

func (server *CameraServer) handleCameraPrivacy(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method == http.MethodGet {
		session := GetTapoSession(server.defaultCameraIP, "", "")
		server.writeJSON(
			writer,
			map[string]interface{}{
				"ok":      true,
				"enabled": session.GetPrivacyMode(),
			},
			200,
		)
		return
	}

	if request.Method != http.MethodPost {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Chỉ hỗ trợ GET hoặc POST"},
			http.StatusMethodNotAllowed,
		)
		return
	}

	var payload CameraPrivacyRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	if payload.Enabled == nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Thiếu tham số enabled"},
			http.StatusBadRequest,
		)
		return
	}

	cameraHost := payload.IP
	if cameraHost == "" {
		cameraHost = server.defaultCameraIP
	}

	effectivePassword := payload.CloudPassword
	if effectivePassword == "" {
		effectivePassword = payload.Password
	}
	if effectivePassword == "" {
		server.writeJSON(
			writer,
			map[string]interface{}{
				"ok":    false,
				"error": "Thiếu mật khẩu Camera (hãy nhập vào Cài Đặt Camera)",
			},
			http.StatusBadRequest,
		)
		return
	}

	session := GetTapoSession(cameraHost, payload.User, effectivePassword)
	privacyErr := session.SetPrivacyMode(*payload.Enabled)
	if privacyErr != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{
				"ok":    false,
				"error": fmt.Sprintf("Lỗi bật/tắt chế độ riêng tư: %v", privacyErr),
			},
			500,
		)
		return
	}

	actionMessage := "Đã tắt chế độ riêng tư camera"
	if *payload.Enabled {
		actionMessage = "Đã bật chế độ riêng tư camera"
	}

	server.writeJSON(
		writer,
		map[string]interface{}{
			"ok":      true,
			"enabled": *payload.Enabled,
			"message": actionMessage,
		},
		200,
	)
}
