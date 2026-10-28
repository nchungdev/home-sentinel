package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/nchungdev/home-sentinel/internal/integrations/camera"
)

func main() {
	webPort := 8792
	if customPort := os.Getenv("CAMERA_WEB_PORT"); customPort != "" {
		if parsed, parseError := strconv.Atoi(customPort); parseError == nil {
			webPort = parsed
		}
	}

	go2rtcPort := 1984
	if customGo2rtc := os.Getenv("GO2RTC_PORT"); customGo2rtc != "" {
		if parsed, parseError := strconv.Atoi(customGo2rtc); parseError == nil {
			go2rtcPort = parsed
		}
	}

	serverInstance := camera.NewCameraServer(webPort, go2rtcPort)
	if startError := serverInstance.Start(); startError != nil {
		fmt.Printf("Server exit error: %v\n", startError)
		os.Exit(1)
	}
}
