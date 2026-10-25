package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/speech"
)

func startCameraWakeWordListener(server *Server) {
	logMic := func(format string, a ...interface{}) {
		msg := fmt.Sprintf("[%s] 🎤 Mic Camera: %s", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
		fmt.Println(msg)
		server.broadcastTerminal(msg)
	}

	go func() {
		logMic("Bắt đầu tiến trình lắng nghe âm thanh nền từ Mic Camera...")
		waitingForConfig := false

		for {
			var cfg CameraConfigData
			data, err := os.ReadFile("sentinel_camera_config.json")
			if err != nil {
				if !waitingForConfig {
					logMic("Chưa có sentinel_camera_config.json; chờ cấu hình...")
					waitingForConfig = true
				}
				time.Sleep(5 * time.Second)
				continue
			}
			if err := json.Unmarshal(data, &cfg); err != nil || cfg.IP == "" || cfg.Username == "" || cfg.Password == "" {
				if !waitingForConfig {
					logMic("Cấu hình camera chưa đủ IP/user/pass; chờ cập nhật...")
					waitingForConfig = true
				}
				time.Sleep(5 * time.Second)
				continue
			}

			waitingForConfig = false
			rtspPath := cfg.StreamPath
			if rtspPath == "" {
				rtspPath = "/stream2"
			}
			rtspURL := fmt.Sprintf("rtsp://%s:%s@%s:554%s", cfg.Username, cfg.Password, cfg.IP, rtspPath)
			cmd := exec.Command("ffmpeg", "-rtsp_transport", "tcp", "-i", rtspURL, "-af", "highpass=f=200,lowpass=f=3400,volume=2.5", "-f", "s16le", "-ar", "16000", "-ac", "1", "-")
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				logMic("Không tạo được luồng PCM; thử lại sau 5 giây.")
				time.Sleep(5 * time.Second)
				continue
			}
			if err := cmd.Start(); err != nil {
				logMic("Không khởi động được FFmpeg; kiểm tra cài đặt/quyền chạy. Thử lại sau 5 giây.")
				time.Sleep(5 * time.Second)
				continue
			}

			logMic("Đã khởi động FFmpeg; đang chờ PCM 16 kHz mono từ camera.")

			buf := make([]byte, 3200)
			var speechBuffer []byte
			silenceChunks := 0
			isSpeaking := false
			lastLevelLog := time.Time{}
			var windowSum float64
			var windowSamples, windowPeak int

			for {
				n, err := io.ReadFull(stdout, buf)
				if err != nil || n < 2 {
					break
				}

				var sum float64
				samples := n / 2
				for i := 0; i < samples; i++ {
					val := int16(buf[2*i]) | (int16(buf[2*i+1]) << 8)
					sum += float64(val) * float64(val)
					peak := int(val)
					if peak < 0 {
						peak = -peak
					}
					if peak > windowPeak {
						windowPeak = peak
					}
				}
				rms := math.Sqrt(sum / float64(samples))
				windowSum += sum
				windowSamples += samples
				if time.Since(lastLevelLog) >= time.Second {
					if rms > 450.0 || isSpeaking || len(speechBuffer) > 0 {
						level := math.Sqrt(windowSum / float64(windowSamples))
						state := "dưới ngưỡng"
						if rms > 450.0 {
							state = "vượt ngưỡng"
						}
						logMic("PCM: RMS=%.0f, peak=%d, %s, thu=%.1fs", level, windowPeak, state, float64(len(speechBuffer))/32000)
						lastLevelLog = time.Now()
					}
					windowSum, windowSamples, windowPeak = 0, 0, 0
				}

				if rms > 450.0 {
					if !isSpeaking {
						logMic("Bắt đầu đoạn âm thanh vượt ngưỡng (RMS=%.1f).", rms)
					}
					isSpeaking = true
					silenceChunks = 0
					speechBuffer = append(speechBuffer, buf[:n]...)
				} else if isSpeaking {
					silenceChunks++
					speechBuffer = append(speechBuffer, buf[:n]...)

					if silenceChunks > 7 {
						isSpeaking = false
						if len(speechBuffer) >= 16000*2*8/10 {
							audioToTranscribe := speechBuffer
							speechBuffer = nil

							audioFeatures := speech.AnalyzeAudioSegment(audioToTranscribe)
							tNow := time.Now().Format("15:04:05")

							if audioFeatures.Category == speech.CategoryClap {
								logMic("👏 [Phát Hiện Tiếng Vỗ Tay] %s! Peaks=%d, PeakVal=%d, RMS=%.0f, ZCR=%.3f",
									audioFeatures.Description, audioFeatures.ClapCount, audioFeatures.Peak,
									audioFeatures.RMS, audioFeatures.ZeroCrossing)
								server.broadcastTerminal(fmt.Sprintf("[%s] 👏 Camera ghi nhận: %s (Peaks: %d, Max: %d)",
									tNow, audioFeatures.Description, audioFeatures.ClapCount, audioFeatures.Peak))
								if server.AutomationMgr != nil {
									server.AutomationMgr.RecordActivity("tiếng vỗ tay")
								}
								continue
							}

							if audioFeatures.Category != speech.CategorySpeech {
								continue
							}

							// Worker pool xử lý STT bất đồng bộ (tối đa 2 worker đồng thời) thay vì block hoàn toàn luồng thu
							server.transcribeLock.Lock()
							if server.transcribeWorkers >= 2 {
								server.transcribeLock.Unlock()
								logMic("Hệ thống STT đang xử lý 2 câu liên tiếp, bỏ qua đoạn này để giải phóng tải.")
								continue
							}
							server.transcribeWorkers++
							server.transcribeLock.Unlock()

							logMic("🗣️ [Âm Thanh Phòng Khách] Quét từ khóa nội bộ Local Whisper (%.1fs, RMS=%.0f, ZCR=%.3f)...",
								float64(len(audioToTranscribe))/32000, audioFeatures.RMS, audioFeatures.ZeroCrossing)

							go func(rawPCM []byte, cfg CameraConfigData) {
								defer func() {
									server.transcribeLock.Lock()
									server.transcribeWorkers--
									server.transcribeLock.Unlock()
								}()

								var text string
								var terr error
								if server.Transcriber != nil {
									text, _, terr = server.Transcriber.TranscribeLocal(rawPCM)
								} else {
									text, terr = speech.TranscribeLocalPCM(rawPCM, "tiny-int8")
								}
								if terr != nil || text == "" {
									return
								}
								logMic("Local Whisper nhận: %q", text)
								lowerT := strings.ToLower(text)
								if isWhisperHallucination(lowerT) {
									return
								}
								// Chuẩn hóa & đoán ý theo context nhà thông minh
								normalizedText := NormalizeSmartHomeQuery(text)
								if normalizedText != text {
									logMic("Đoán ý thông minh: %q -> %q", text, normalizedText)
									text = normalizedText
								}

								tNow := time.Now().Format("15:04:05")
								if server.AutomationMgr != nil {
									server.AutomationMgr.RecordActivity("tiếng nói / âm thanh phòng khách")
									if server.AutomationMgr.MatchesDoorSound(text) {
										go server.AutomationMgr.HandleEveningArrival("âm thanh mở cửa / lời chào về nhà")
									}
								}

								matched, query := matchesWakeWord(text)
								if !matched {
									// Người trong phòng đang nói chuyện phiếm hoặc xem TV mà không gọi Duinch -> Bỏ qua, KHÔNG gửi Groq!
									return
								}

								logMic("🎯 [WakeWord Hợp Lệ] Đã phát hiện từ khóa gọi Duinch! Query: %q", query)
								fmt.Printf("[Mic Camera Heard]: \"%s\"\n", text)
								server.broadcastTerminal(fmt.Sprintf("[%s] 🎤 Camera Mic thu được: \"%s\"", tNow, text))

								server.handleWakeWordQuery(query, cfg, tNow)
							}(audioToTranscribe, cfg)
						} else {
							logMic("Bỏ đoạn âm thanh quá ngắn: %.2fs.", float64(len(speechBuffer))/32000)
							speechBuffer = nil
						}
					}
				}
			}

			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			logMic("Luồng PCM đã ngắt; kết nối lại sau 3 giây.")
			time.Sleep(3 * time.Second)
		}
	}()
}

func (s *Server) handleWakeWordQuery(query string, cfg CameraConfigData, tNow string) {
	fmt.Printf("🎯 [WakeWord Triggered] Duinch! Query: \"%s\"\n", query)
	s.broadcastTerminal(fmt.Sprintf("[%s] 🎯 Duinch đã nhận diện từ khóa đánh thức! Lệnh: \"%s\"", tNow, query))

	if query == "" {
		s.broadcastTerminal(fmt.Sprintf("[%s] 🗣️ Duinch Camera phản hồi: \"Dạ, em nghe đây ạ!\"", tNow))
		_ = s.speakOnCamera("Dạ, em nghe đây ạ!", "female")
		time.Sleep(2 * time.Second)
		// Thu âm câu lệnh tiếp theo và gửi sang Groq Whisper Large-v3 để nhận diện siêu chính xác
		followUp, ferr := s.recordAndTranscribeCloud(cfg, 4)
		if ferr == nil && followUp != "" {
			query = followUp
			s.broadcastTerminal(fmt.Sprintf("[%s] 🎤 User nói tiếp: \"%s\"", time.Now().Format("15:04:05"), query))
		}
	}

	if query != "" {
		lowerQuery := strings.ToLower(query)
		if strings.Contains(lowerQuery, "mở tv") || strings.Contains(lowerQuery, "bật tv") {
			s.broadcastTerminal(fmt.Sprintf("[%s] 📺 Duinch Camera: \"Dạ, em mở TV ngay đây ạ!\"", time.Now().Format("15:04:05")))
			_ = s.speakOnCamera("Dạ, em mở TV ngay đây ạ!", "female")
			_ = turnOnTV("192.168.1.8")
		} else if strings.Contains(lowerQuery, "tắt tv") {
			s.broadcastTerminal(fmt.Sprintf("[%s] 📺 Duinch Camera: \"Dạ, em tắt TV đây ạ!\"", time.Now().Format("15:04:05")))
			_ = s.speakOnCamera("Dạ, em tắt TV đây ạ!", "female")
			adbBin := "/home/chungnh/bin/adb"
			_ = exec.Command(adbBin, "-s", "192.168.1.8:5555", "shell", "input keyevent 26").Run()
		} else {
			reply := askAgentQuery(s, query)
			s.broadcastTerminal(fmt.Sprintf("[%s] 🤖 Duinch Camera trả lời: \"%s\"", time.Now().Format("15:04:05"), reply))
			_ = s.speakOnCamera(reply, "female")
		}
	}
}

func askAgentQuery(server *Server, question string) string {
	orch := buildServerOrchestrator(server)
	reqID := fmt.Sprintf("cam-%d", time.Now().UnixNano())
	inputEv := &contracts.InputEvent{
		RequestID: reqID,
		SessionID: "camera_intercom",
		Text:      question,
		Locale:    "vi-VN",
		InputType: "voice",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pres, err := orch.Process(ctx, inputEv, nil)
	if err != nil || pres == nil || pres.SpeechText == "" {
		return "Em đây ạ, anh cần giúp gì thế anh?"
	}

	return pres.SpeechText
}
