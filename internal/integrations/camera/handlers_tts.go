package camera

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (server *CameraServer) handleTtsPreview(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload TtsPreviewRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	if strings.TrimSpace(payload.Text) == "" {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Thiếu nội dung"},
			http.StatusBadRequest,
		)
		return
	}

	audioData, generateError := GenerateSpeechAudio(
		payload.Text, payload.Voice, 0, payload.Speed,
	)
	if generateError != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": generateError.Error()},
			http.StatusInternalServerError,
		)
		return
	}

	writer.Header().Set("Content-Type", "audio/wav")
	writer.Header().Set("Content-Length", strconv.Itoa(len(audioData)))
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(audioData)
}

func (server *CameraServer) handleSpeak(
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

	var payload SpeakRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Dữ liệu JSON không hợp lệ"},
			http.StatusBadRequest,
		)
		return
	}

	if strings.TrimSpace(payload.Text) == "" {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Vui lòng nhập nội dung muốn phát ra loa"},
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
			map[string]interface{}{"ok": false, "error": "Chưa có mật khẩu Camera"},
			http.StatusBadRequest,
		)
		return
	}

	audioWavData, ttsErr := GenerateSpeechAudio(
		payload.Text, payload.Voice, 0, payload.Speed,
	)
	if ttsErr != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": fmt.Sprintf("Lỗi TTS: %v", ttsErr)},
			http.StatusInternalServerError,
		)
		return
	}

	broadcastErr := BroadcastAudio(
		cameraHost, payload.User, effectivePassword, audioWavData,
	)
	if broadcastErr != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{
				"ok":      false,
				"error":   fmt.Sprintf("Lỗi Tapo speaker (8800): %v", broadcastErr),
				"message": nil,
			},
			200,
		)
		return
	}

	server.writeJSON(
		writer,
		map[string]interface{}{
			"ok":      true,
			"message": "Phát loa thành công qua cổng 8800",
			"error":   nil,
		},
		200,
	)
}
