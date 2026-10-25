package camera

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

func (server *CameraServer) handleTestCamera(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload RtspTestRequest
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		server.writeJSON(writer, map[string]interface{}{"ok": false, "error": "Invalid JSON"}, 400)
		return
	}

	cameraHost := payload.IP
	if cameraHost == "" {
		cameraHost = server.defaultCameraIP
	}
	cameraPort := payload.Port
	if cameraPort <= 0 {
		cameraPort = server.defaultCameraPort
	}
	streamPath := payload.Path
	if streamPath == "" {
		streamPath = server.defaultStreamPath
	}

	success, detail := testRtspConnection(
		cameraHost, cameraPort, payload.User, payload.Password, streamPath,
	)
	responsePayload := map[string]interface{}{
		"ok":      success,
		"message": detail,
		"error":   nil,
	}
	if !success {
		responsePayload["message"] = nil
		responsePayload["error"] = detail
	}
	server.writeJSON(writer, responsePayload, http.StatusOK)
}

func (server *CameraServer) handleRegisterStream(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload RtspTestRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	cameraHost := payload.IP
	if cameraHost == "" {
		cameraHost = server.defaultCameraIP
	}
	cameraPort := payload.Port
	if cameraPort <= 0 {
		cameraPort = server.defaultCameraPort
	}
	streamPath := payload.Path
	if streamPath == "" {
		streamPath = server.defaultStreamPath
	}

	cleanPath := streamPath
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	sourceUrl := fmt.Sprintf(
		"rtsp://%s:%s@%s:%d%s",
		payload.User, payload.Password, cameraHost, cameraPort, cleanPath,
	)
	registerEndpoint := fmt.Sprintf(
		"http://127.0.0.1:%d/api/streams?src=camera_live&dst=%s",
		server.go2rtcPort, url.QueryEscape(sourceUrl),
	)

	putRequest, _ := http.NewRequest(http.MethodPut, registerEndpoint, nil)
	client := http.Client{Timeout: 3 * time.Second}
	resp, requestError := client.Do(putRequest)
	registered := requestError == nil && resp.StatusCode < 300
	if resp != nil {
		_ = resp.Body.Close()
	}

	server.writeJSON(writer, map[string]interface{}{"ok": true, "registered": registered}, 200)
}

func (server *CameraServer) handleOpenFfplay(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload RtspTestRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	cameraHost := payload.IP
	if cameraHost == "" {
		cameraHost = server.defaultCameraIP
	}
	cameraPort := payload.Port
	if cameraPort <= 0 {
		cameraPort = server.defaultCameraPort
	}
	streamPath := payload.Path
	if streamPath == "" {
		streamPath = server.defaultStreamPath
	}
	cleanPath := streamPath
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	rtspUrl := fmt.Sprintf(
		"rtsp://%s:%s@%s:%d%s",
		payload.User, payload.Password, cameraHost, cameraPort, cleanPath,
	)

	ffplayProcess := exec.Command(
		"ffplay",
		"-window_title", fmt.Sprintf("Live Camera - %s", cameraHost),
		"-rtsp_transport", "tcp",
		rtspUrl,
	)
	if launchError := ffplayProcess.Start(); launchError != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": launchError.Error()},
			500,
		)
		return
	}

	server.writeJSON(
		writer,
		map[string]interface{}{"ok": true, "message": "Đã mở cửa sổ ffplay"},
		200,
	)
}

func (server *CameraServer) handleStreamMjpeg(
	writer http.ResponseWriter,
	request *http.Request,
) {
	query := request.URL.Query()
	cameraHost := query.Get("ip")
	if cameraHost == "" {
		cameraHost = server.defaultCameraIP
	}
	cameraPort := query.Get("port")
	if cameraPort == "" {
		cameraPort = "554"
	}
	cameraUser := query.Get("user")
	cameraPassword := query.Get("password")
	streamPath := query.Get("path")
	if streamPath == "" {
		streamPath = server.defaultStreamPath
	}
	cleanPath := streamPath
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	rtspUrl := fmt.Sprintf(
		"rtsp://%s:%s@%s:%s%s",
		cameraUser, cameraPassword, cameraHost, cameraPort, cleanPath,
	)

	ffmpegProcess := exec.Command(
		"ffmpeg",
		"-rtsp_transport", "tcp",
		"-i", rtspUrl,
		"-f", "mpjpeg",
		"-q:v", "5",
		"-r", "12",
		"pipe:1",
	)

	stdoutPipe, pipeError := ffmpegProcess.StdoutPipe()
	if pipeError != nil {
		http.Error(writer, pipeError.Error(), http.StatusInternalServerError)
		return
	}
	if startError := ffmpegProcess.Start(); startError != nil {
		http.Error(writer, startError.Error(), http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = ffmpegProcess.Process.Kill()
		_ = ffmpegProcess.Wait()
	}()

	writer.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=--ffmpeg")
	writer.WriteHeader(http.StatusOK)

	buffer := make([]byte, 4096)
	for {
		readBytes, readErr := stdoutPipe.Read(buffer)
		if readBytes > 0 {
			if _, writeErr := writer.Write(buffer[:readBytes]); writeErr != nil {
				break
			}
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if readErr != nil {
			break
		}
	}
}
