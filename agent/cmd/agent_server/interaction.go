package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/camstream"
)

func (s *Server) speakOnCamera(text string, voice string) error {
	if voice == "" {
		voice = "female"
	}
	var cfg CameraConfigData
	if data, err := os.ReadFile("sentinel_camera_config.json"); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg.IP == "" {
		return fmt.Errorf("chưa cấu hình IP camera")
	}

	effectivePass := cfg.CloudPassword
	if effectivePass == "" {
		effectivePass = cfg.Password
	}

	speakerBin := "./bin/tapo_speaker"
	if _, err := os.Stat(speakerBin); err != nil {
		speakerBin = "/home/chungnh/home-sentinel/bin/tapo_speaker"
	}

	cmd := exec.Command(speakerBin,
		"-ip", cfg.IP,
		"-user", cfg.Username,
		"-password", effectivePass,
		"-text", text,
		"-voice", voice,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, string(out))
	}
	return nil
}

func (s *Server) handleProactiveSofaPrompt() {
	tvHost := "192.168.1.8"
	s.promptMutex.Lock()
	if time.Since(s.lastTVPromptTime) < 15*time.Minute {
		s.promptMutex.Unlock()
		return
	}
	if isTVScreenOn(tvHost) {
		s.promptMutex.Unlock()
		return
	}
	s.lastTVPromptTime = time.Now()
	s.promptMutex.Unlock()

	var cfg CameraConfigData
	if data, err := os.ReadFile("sentinel_camera_config.json"); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	effectivePass := cfg.CloudPassword
	if effectivePass == "" {
		effectivePass = cfg.Password
	}

	if cfg.IP != "" && effectivePass != "" {
		session := camstream.GetTapoSession(cfg.IP, cfg.Username, effectivePass)
		_ = session.GotoPreset("1") // Token 1 = Sofa
	}

	time.Sleep(2 * time.Second)

	promptText := "Anh có muốn xem TV không? Em mở giúp anh nhé"
	s.broadcastTerminal(fmt.Sprintf("[%s] 🗣️ Duinch Camera chủ động hỏi: \"%s\"", time.Now().Format("15:04:05"), promptText))
	_ = s.speakOnCamera(promptText, "female")

	time.Sleep(3 * time.Second)
	reply, err := recordAndTranscribe(cfg, 3)
	if err == nil && reply != "" {
		lowerReply := strings.ToLower(reply)
		tReply := time.Now().Format("15:04:05")
		s.broadcastTerminal(fmt.Sprintf("[%s] 🎤 User trả lời camera: \"%s\"", tReply, reply))

		positiveIntent := strings.Contains(lowerReply, "ok") ||
			strings.Contains(lowerReply, "mở") ||
			strings.Contains(lowerReply, "bật") ||
			strings.Contains(lowerReply, "có") ||
			strings.Contains(lowerReply, "ừ") ||
			strings.Contains(lowerReply, "xem") ||
			strings.Contains(lowerReply, "yes") ||
			strings.Contains(lowerReply, "được")

		if positiveIntent {
			s.broadcastTerminal(fmt.Sprintf("[%s] 📺 Duinch Camera: \"Dạ, em mở TV ngay đây ạ!\"", time.Now().Format("15:04:05")))
			_ = s.speakOnCamera("Dạ, em mở TV ngay đây ạ!", "female")
			_ = turnOnTV(tvHost)
		}
	}
}

func isTVScreenOn(tvHost string) bool {
	adbBin := "/home/chungnh/bin/adb"
	if _, err := os.Stat(adbBin); err != nil {
		adbBin = "adb"
	}
	target := tvHost
	if !strings.Contains(target, ":") {
		target = target + ":5555"
	}

	cmd := exec.Command(adbBin, "-s", target, "shell", "dumpsys power | grep mWakefulness=")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "mWakefulness=Awake")
}

func turnOnTV(tvHost string) error {
	adbBin := "/home/chungnh/bin/adb"
	if _, err := os.Stat(adbBin); err != nil {
		adbBin = "adb"
	}
	target := tvHost
	if !strings.Contains(target, ":") {
		target = target + ":5555"
	}
	_ = exec.Command(adbBin, "connect", target).Run()
	_ = exec.Command(adbBin, "-s", target, "shell", "input keyevent 224").Run()
	_ = exec.Command(adbBin, "-s", target, "shell", "input keyevent 3").Run()
	return nil
}

func recordAndTranscribe(cfg CameraConfigData, durationSec int) (string, error) {
	if cfg.IP == "" {
		return "", fmt.Errorf("chưa cấu hình IP camera")
	}
	tmpWav := fmt.Sprintf("/tmp/cam_rec_%d.wav", time.Now().UnixNano())
	defer os.Remove(tmpWav)

	rtspPath := cfg.StreamPath
	if rtspPath == "" {
		rtspPath = "/stream2"
	}
	rtspURL := fmt.Sprintf("rtsp://%s:%s@%s:554%s", cfg.Username, cfg.Password, cfg.IP, rtspPath)
	ffmpegCmd := exec.Command("ffmpeg", "-y", "-rtsp_transport", "tcp", "-i", rtspURL, "-t", strconv.Itoa(durationSec), "-vn", "-ar", "16000", "-ac", "1", tmpWav)
	if err := ffmpegCmd.Run(); err != nil {
		return "", fmt.Errorf("lỗi ffmpeg: %w", err)
	}

	pcmData, err := os.ReadFile(tmpWav)
	if err != nil {
		return "", fmt.Errorf("lỗi đọc file wav: %w", err)
	}
	if len(pcmData) > 44 {
		pcmData = pcmData[44:]
	}

	return transcribePCM(pcmData)
}
