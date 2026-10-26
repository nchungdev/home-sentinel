package camera

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
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

func NewCameraServer(webPort int, go2rtcPort int) *CameraServer {
	defaultIP := os.Getenv("CAMERA_IP")
	if defaultIP == "" {
		defaultIP = "192.168.1.100"
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
	mux.HandleFunc("/api/events", server.handleCameraEvent)

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
	}, http.StatusOK)
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
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	writer.WriteHeader(statusCode)
	_, _ = writer.Write(encoded)
}

func (server *CameraServer) ensureGo2rtcRunning() {
	client := http.Client{Timeout: 1 * time.Second}
	probeUrl := fmt.Sprintf("http://127.0.0.1:%d/api/streams", server.go2rtcPort)
	resp, requestError := client.Get(probeUrl)
	if requestError == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		return
	}

	binaryPaths := []string{
		"./bin/go2rtc",
		"/home/chungnh/bin/go2rtc",
		"/usr/local/bin/go2rtc",
	}

	for _, binaryLocation := range binaryPaths {
		if _, statError := os.Stat(binaryLocation); statError == nil {
			go2rtcProcess := exec.Command(binaryLocation)
			_ = go2rtcProcess.Start()
			time.Sleep(500 * time.Millisecond)
			return
		}
	}
}

func testRtspConnection(
	host string,
	port int,
	username string,
	password string,
	streamPath string,
) (bool, string) {
	connectionAddress := fmt.Sprintf("%s:%d", host, port)
	tcpConn, tcpError := net.DialTimeout("tcp", connectionAddress, 3*time.Second)
	if tcpError != nil {
		return false, fmt.Sprintf("Không thể kết nối cổng %d: %v", port, tcpError)
	}
	defer tcpConn.Close()

	cleanPath := streamPath
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}
	rtspUrl := fmt.Sprintf("rtsp://%s:%d%s", host, port, cleanPath)
	describeMessage := fmt.Sprintf(
		"DESCRIBE %s RTSP/1.0\r\nCSeq: 1\r\nAccept: application/sdp\r\n\r\n",
		rtspUrl,
	)

	_ = tcpConn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, writeErr := tcpConn.Write([]byte(describeMessage)); writeErr != nil {
		return false, fmt.Sprintf("Gửi DESCRIBE thất bại: %v", writeErr)
	}

	readBuffer := make([]byte, 2048)
	bytesRead, readError := tcpConn.Read(readBuffer)
	if readError != nil {
		return false, fmt.Sprintf("Đọc RTSP thất bại: %v", readError)
	}

	responseString := string(readBuffer[:bytesRead])
	if strings.Contains(responseString, "401 Unauthorized") {
		authLine := ""
		for _, headerLine := range strings.Split(responseString, "\r\n") {
			if strings.HasPrefix(strings.ToLower(headerLine), "www-authenticate:") {
				authLine = headerLine
				break
			}
		}

		calculatedAuth := computeDigestAuthHeader(username, password, "DESCRIBE", rtspUrl, authLine)
		if calculatedAuth != "" {
			secondDescribeMessage := fmt.Sprintf(
				"DESCRIBE %s RTSP/1.0\r\nCSeq: 2\r\nAccept: application/sdp\r\nAuthorization: %s\r\n\r\n",
				rtspUrl, calculatedAuth,
			)

			reconnectedTcp, reconnectError := net.DialTimeout("tcp", connectionAddress, 3*time.Second)
			if reconnectError == nil {
				defer reconnectedTcp.Close()
				_ = reconnectedTcp.SetDeadline(time.Now().Add(3 * time.Second))
				_, _ = reconnectedTcp.Write([]byte(secondDescribeMessage))
				secondBytesRead, secondReadError := reconnectedTcp.Read(readBuffer)
				if secondReadError == nil {
					secondResponseString := string(readBuffer[:secondBytesRead])
					if strings.Contains(secondResponseString, "200 OK") {
						return true, "Kết nối thành công (Digest Auth OK)"
					}
					if strings.Contains(secondResponseString, "401") {
						return false, "Xác thực thất bại: Sai tài khoản hoặc mật khẩu Camera"
					}
				}
			}
		}

		return true, "Cổng RTSP hoạt động (Camera phản hồi 401 Unauthorized)"
	}

	if strings.Contains(responseString, "200 OK") {
		return true, "Kết nối RTSP thành công (200 OK)"
	}

	return true, "Cổng RTSP phản hồi thành công"
}

func computeDigestAuthHeader(user, pass, method, rawURL, wwwAuth string) string {
	realm := extractDigestParam(wwwAuth, "realm")
	nonce := extractDigestParam(wwwAuth, "nonce")
	if realm == "" || nonce == "" {
		return ""
	}
	parsedURL, _ := url.Parse(rawURL)
	uri := rawURL
	if parsedURL != nil && parsedURL.RequestURI() != "" {
		uri = parsedURL.RequestURI()
	}
	ha1 := md5Hex(fmt.Sprintf("%s:%s:%s", user, realm, pass))
	ha2 := md5Hex(fmt.Sprintf("%s:%s", method, uri))
	response := md5Hex(fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2))
	return fmt.Sprintf("Digest username=\"%s\", realm=\"%s\", nonce=\"%s\", uri=\"%s\", response=\"%s\"", user, realm, nonce, uri, response)
}

func extractDigestParam(header, key string) string {
	search := key + "=\""
	idx := strings.Index(header, search)
	if idx == -1 {
		return ""
	}
	start := idx + len(search)
	end := strings.Index(header[start:], "\"")
	if end == -1 {
		return ""
	}
	return header[start : start+end]
}

func contextBackground() context.Context {
	return context.Background()
}
