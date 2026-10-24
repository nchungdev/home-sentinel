package camera

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type CameraServer struct {
	webPort           int
	go2rtcPort        int
	defaultCameraIP   string
	defaultCameraPort int
	defaultStreamPath string
	sentinel          *CameraSentinel
	httpServer        *http.Server
}

type RtspTestRequest struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Path     string `json:"path"`
}

type SpeakRequest struct {
	IP            string  `json:"ip"`
	Port          int     `json:"port"`
	User          string  `json:"user"`
	Password      string  `json:"password"`
	CloudPassword string  `json:"cloudPassword"`
	Path          string  `json:"path"`
	Text          string  `json:"text"`
	Voice         string  `json:"voice"`
	Speed         float64 `json:"speed"`
}

type TtsPreviewRequest struct {
	Text  string  `json:"text"`
	Voice string  `json:"voice"`
	Speed float64 `json:"speed"`
}

type SecurityConfigRequest struct {
	AlarmSpeaker *bool `json:"alarmSpeaker"`
	AlarmTv      *bool `json:"alarmTv"`
}

type WhitelistIPRequest struct {
	IP string `json:"ip"`
}

type CameraPTZRequest struct {
	IP            string `json:"ip"`
	Direction     string `json:"direction"`
	Step          int    `json:"step"`
	X             int    `json:"x"`
	Y             int    `json:"y"`
	User          string `json:"user"`
	Password      string `json:"password"`
	CloudPassword string `json:"cloudPassword"`
}

type CameraPrivacyRequest struct {
	IP            string `json:"ip"`
	Enabled       *bool  `json:"enabled"`
	User          string `json:"user"`
	Password      string `json:"password"`
	CloudPassword string `json:"cloudPassword"`
}

func NewCameraServer(webPort int, go2rtcPort int) *CameraServer {
	defaultIP := os.Getenv("CAMERA_IP")
	if defaultIP == "" {
		defaultIP = "192.168.1.6"
	}

	return &CameraServer{
		webPort:           webPort,
		go2rtcPort:        go2rtcPort,
		defaultCameraIP:   defaultIP,
		defaultCameraPort: 554,
		defaultStreamPath: "/stream1",
		sentinel:          NewCameraSentinel(go2rtcPort),
	}
}

func (server *CameraServer) Start() error {
	server.ensureGo2rtcRunning()
	server.sentinel.Start(contextBackground())

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)
	mux.HandleFunc("/api/test-camera", server.handleTestCamera)
	mux.HandleFunc("/api/camera/register-stream", server.handleRegisterStream)
	mux.HandleFunc("/api/camera/open-ffplay", server.handleOpenFfplay)
	mux.HandleFunc("/api/camera/ptz", server.handleCameraPTZ)
	mux.HandleFunc("/api/camera/privacy", server.handleCameraPrivacy)
	mux.HandleFunc("/api/tts/preview", server.handleTtsPreview)
	mux.HandleFunc("/api/camera/speak", server.handleSpeak)
	mux.HandleFunc("/api/security/status", server.handleSecurityStatus)
	mux.HandleFunc("/api/security/config", server.handleSecurityConfig)
	mux.HandleFunc("/api/security/whitelist/add", server.handleWhitelistAdd)
	mux.HandleFunc("/api/security/whitelist/remove", server.handleWhitelistRemove)
	mux.HandleFunc("/api/security/test-alarm", server.handleSecurityTestAlarm)
	mux.HandleFunc("/api/monitor/realtime", server.handleMonitorRealtime)
	mux.HandleFunc("/api/camera/stream.mjpeg", server.handleStreamMjpeg)

	server.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", server.webPort),
		Handler: mux,
	}

	fmt.Printf("Camera Command Center (Go) running at http://localhost:%d\n", server.webPort)
	return server.httpServer.ListenAndServe()
}

func (server *CameraServer) handleIndex(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" && request.URL.Path != "/index.html" {
		http.NotFound(writer, request)
		return
	}
	server.writeJSON(writer, map[string]interface{}{
		"service": "camera-service",
		"status":  "online",
		"hub_url": "http://localhost:8888",
	}, http.StatusOK)
}

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
			500,
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
	var payload SpeakRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	if strings.TrimSpace(payload.Text) == "" {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Thiếu nội dung"},
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

	audioData, generateError := GenerateSpeechAudio(
		payload.Text, payload.Voice, 600, payload.Speed,
	)
	if generateError != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": generateError.Error()},
			500,
		)
		return
	}

	broadcastError := BroadcastAudio(
		cameraHost, payload.User, effectivePassword, audioData,
	)
	if broadcastError != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{
				"ok":      false,
				"error":   fmt.Sprintf("Lỗi Tapo speaker (8800): %v", broadcastError),
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

func (server *CameraServer) writeJSON(
	writer http.ResponseWriter,
	payload interface{},
	statusCode int,
) {
	encoded, jsonError := json.Marshal(payload)
	if jsonError != nil {
		http.Error(writer, jsonError.Error(), 500)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Content-Length", strconv.Itoa(len(encoded)))
	writer.WriteHeader(statusCode)
	_, _ = writer.Write(encoded)
}

func (server *CameraServer) ensureGo2rtcRunning() {
	checkUrl := fmt.Sprintf("http://127.0.0.1:%d/api/streams", server.go2rtcPort)
	client := http.Client{Timeout: 800 * time.Millisecond}
	resp, requestError := client.Get(checkUrl)
	if requestError == nil && resp.StatusCode == 200 {
		_ = resp.Body.Close()
		return
	}

	go2rtcBinary, lookError := exec.LookPath("go2rtc")
	if lookError != nil {
		homeDir, _ := os.UserHomeDir()
		go2rtcBinary = filepath.Join(homeDir, "go", "bin", "go2rtc")
	}
	if _, statError := os.Stat(go2rtcBinary); statError == nil {
		daemonProcess := exec.Command(
			go2rtcBinary,
			"-c", fmt.Sprintf(`{"api":{"listen":":%d"}}`, server.go2rtcPort),
		)
		_ = daemonProcess.Start()
	}
}

func testRtspConnection(
	host string,
	port int,
	user string,
	password string,
	path string,
) (bool, string) {
	targetAddress := fmt.Sprintf("%s:%d", host, port)
	socketConnection, dialError := net.DialTimeout("tcp", targetAddress, 4*time.Second)
	if dialError != nil {
		return false, fmt.Sprintf("Lỗi kết nối tới %s: %v", targetAddress, dialError)
	}
	defer socketConnection.Close()

	authHeader := ""
	if user != "" && password != "" {
		rawCredentials := fmt.Sprintf("%s:%s", user, password)
		encodedToken := base64.StdEncoding.EncodeToString([]byte(rawCredentials))
		authHeader = fmt.Sprintf("Authorization: Basic %s\r\n", encodedToken)
	}

	cleanPath := path
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	rtspRequest := fmt.Sprintf(
		"DESCRIBE rtsp://%s:%d%s RTSP/1.0\r\nCSeq: 2\r\n%sAccept: application/sdp\r\n\r\n",
		host, port, cleanPath, authHeader,
	)
	if _, writeError := socketConnection.Write([]byte(rtspRequest)); writeError != nil {
		return false, fmt.Sprintf("Lỗi gửi RTSP request: %v", writeError)
	}

	responseBuffer := make([]byte, 4096)
	_ = socketConnection.SetReadDeadline(time.Now().Add(4 * time.Second))
	readCount, readError := socketConnection.Read(responseBuffer)
	if readError != nil {
		return false, fmt.Sprintf("Lỗi nhận RTSP response: %v", readError)
	}

	firstLine := strings.Split(string(responseBuffer[:readCount]), "\r\n")[0]
	if strings.Contains(firstLine, "200 OK") {
		return true, "Kết nối thành công (200 OK)"
	} else if strings.Contains(firstLine, "401") {
		return false, "Sai tài khoản hoặc mật khẩu camera (401 Unauthorized)"
	} else if strings.Contains(firstLine, "404") {
		return false, fmt.Sprintf("Không tìm thấy luồng %s (404 Not Found)", cleanPath)
	}
	return false, fmt.Sprintf("Phản hồi RTSP: %s", firstLine)
}

func contextBackground() context.Context {
	return context.Background()
}
