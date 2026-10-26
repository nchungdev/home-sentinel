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
	"github.com/nchungdev/home-sentinel/agent/pkg/speech"
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

func (s *Server) executeSmartHomeAction(query string, tNow string) {
	lowerQuery := strings.ToLower(query)
	targetTV := os.Getenv("TV_HOST")
	if targetTV == "" {
		targetTV = "192.168.1.50"
	}
	if strings.Contains(lowerQuery, "mở tv") || strings.Contains(lowerQuery, "bật tv") {
		s.broadcastTerminal(fmt.Sprintf("[%s] 📺 Duinch Camera: \"Dạ, em mở TV ngay đây ạ!\"", tNow))
		_ = s.speakOnCamera("Dạ, em mở TV ngay đây ạ!", "female")
		_ = turnOnTV(targetTV)
	} else if strings.Contains(lowerQuery, "tắt tv") {
		s.broadcastTerminal(fmt.Sprintf("[%s] 📺 Duinch Camera: \"Dạ, em tắt TV đây ạ!\"", tNow))
		_ = s.speakOnCamera("Dạ, em tắt TV đây ạ!", "female")
		_ = turnOffTV(targetTV)
	}
}

func (s *Server) handleProactiveSofaPrompt() {
	s.StateMutex.Lock()
	s.UserOnSofa = true
	s.StateMutex.Unlock()

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

	// Đặt timer tự động hủy UserOnSofa sau 3 phút nếu không có hành động tiếp theo
	go func() {
		time.Sleep(3 * time.Minute)
		s.StateMutex.Lock()
		s.UserOnSofa = false
		s.StateMutex.Unlock()
	}()
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

func turnOffTV(tvHost string) error {
	adbBin := "/home/chungnh/bin/adb"
	if _, err := os.Stat(adbBin); err != nil {
		adbBin = "adb"
	}
	target := tvHost
	if !strings.Contains(target, ":") {
		target = target + ":5555"
	}
	_ = exec.Command(adbBin, "connect", target).Run()
	_ = exec.Command(adbBin, "-s", target, "shell", "input keyevent 223").Run()
	return nil
}

func (s *Server) recordAndTranscribeCloud(cfg CameraConfigData, durationSec int) (string, error) {
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

	if s.Transcriber != nil {
		text, _, err := s.Transcriber.TranscribeCloud(pcmData)
		return text, err
	}
	return speech.TranscribeLocalPCM(pcmData, "tiny-int8")
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

	return speech.TranscribeLocalPCM(pcmData, "tiny-int8")
}
