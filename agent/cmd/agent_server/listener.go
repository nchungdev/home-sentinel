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
			if err != nil || json.Unmarshal(data, &cfg) != nil || cfg.IP == "" || cfg.Username == "" || cfg.Password == "" {
				if !waitingForConfig {
					logMic("Chưa có cấu hình camera hợp lệ; chờ cập nhật...")
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
					if rms > 250.0 || isSpeaking || len(speechBuffer) > 0 {
						level := math.Sqrt(windowSum / float64(windowSamples))
						state := "dưới ngưỡng"
						if rms > 250.0 {
							state = "vượt ngưỡng"
						}
						logMic("PCM: RMS=%.0f, peak=%d, %s, thu=%.1fs", level, windowPeak, state, float64(len(speechBuffer))/32000)
						lastLevelLog = time.Now()
					}
					windowSum, windowSamples, windowPeak = 0, 0, 0
				}

				if rms > 250.0 {
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

							// 1. Kiểm tra qua AI Audio Classifier (YAMNet) trước nếu khả dụng
							yamRes, yerr := speech.ClassifyYAMNet(audioToTranscribe)
							if yerr == nil && yamRes != nil && yamRes.TopLabel != "" {
								logMic("🧠 [YAMNet AI] %s (%.0f%%), Speech=%v, Alarm=%v, Clap=%v",
									yamRes.TopLabel, yamRes.TopScore*100, yamRes.IsSpeech, yamRes.IsAlarm, yamRes.IsClap)

								if yamRes.IsAlarm {
									audioFeatures.Category = speech.CategoryAlarm
									audioFeatures.Description = "Còi báo cháy (" + yamRes.TopLabel + ")"
								} else if yamRes.IsClap {
									audioFeatures.Category = speech.CategoryClap
									audioFeatures.Description = "Tiếng vỗ tay (" + yamRes.TopLabel + ")"
								} else if yamRes.IsSpeech {
									audioFeatures.Category = speech.CategorySpeech
								} else if yamRes.TopScore > 0.40 {
									logMic("Bỏ qua âm thanh môi trường: %s (%.0f%%)", yamRes.TopLabel, yamRes.TopScore*100)
									continue
								}
							}

							if audioFeatures.Category == speech.CategoryClap {
								logMic("👏 [Phát Hiện Tiếng Vỗ Tay] %s (Peaks=%d, Max=%d)", audioFeatures.Description, audioFeatures.ClapCount, audioFeatures.Peak)
								server.broadcastTerminal(fmt.Sprintf("[%s] 👏 Camera: %s", tNow, audioFeatures.Description))
								if server.AutomationMgr != nil {
									server.AutomationMgr.RecordActivity("tiếng vỗ tay")
								}
								continue
							}

							if audioFeatures.Category == speech.CategoryAlarm {
								logMic("🚨 [CẢNH BÁO KHẨN CẤP] %s (RMS=%.0f, Peak=%d)", audioFeatures.Description, audioFeatures.RMS, audioFeatures.Peak)
								server.broadcastTerminal(fmt.Sprintf("[%s] 🚨 CẢNH BÁO: Phát hiện %s!", tNow, audioFeatures.Description))
								if server.AutomationMgr != nil {
									server.AutomationMgr.RecordActivity("còi báo động / báo cháy")
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
									server.AutomationMgr.RecordActivity("âm thanh phòng khách")
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

								server.handleWakeWordQuery(query, rawPCM, cfg, tNow)
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

func (s *Server) handleWakeWordQuery(query string, rawPCM []byte, cfg CameraConfigData, tNow string) {
	fmt.Printf("🎯 [WakeWord Triggered] Duinch! Query: \"%s\"\n", query)
	s.broadcastTerminal(fmt.Sprintf("[%s] 🎯 Duinch đã nhận diện từ khóa đánh thức!", tNow))

	// TRƯỜNG HỢP 1: User chỉ gọi "Duinch ơi" rồi dừng lại
	if query == "" {
		s.broadcastTerminal(fmt.Sprintf("[%s] 🗣️ Duinch Camera phản hồi: \"Dạ, em nghe đây ạ!\"", tNow))
		_ = s.speakOnCamera("Dạ, em nghe đây ạ!", "female")
		time.Sleep(1500 * time.Millisecond)
		// Thu âm câu lệnh tiếp theo và gửi sang Groq Whisper Large-v3 để nhận diện siêu chính xác
		followUp, ferr := s.recordAndTranscribeCloud(cfg, 4)
		if ferr == nil && followUp != "" {
			query = NormalizeSmartHomeQuery(followUp)
			s.broadcastTerminal(fmt.Sprintf("[%s] 🎤 User nói tiếp: \"%s\"", time.Now().Format("15:04:05"), query))
		}
	} else {
		// TRƯỜNG HỢP 2: User nói liền mạch luôn (vd: "Duinch ơi bật đèn")
		// Gửi ngay toàn bộ rawPCM đoạn này sang Groq Whisper Large-v3 để giải mã nguyên vẹn câu lệnh
		if s.Transcriber != nil && s.Transcriber.HasGroqKey() {
			cloudText, _, err := s.Transcriber.TranscribeCloud(rawPCM)
			if err == nil && cloudText != "" {
				s.broadcastTerminal(fmt.Sprintf("[%s] ☁️ Groq Whisper nhận diện chính xác: \"%s\"", tNow, cloudText))
				_, cloudQuery := matchesWakeWord(cloudText)
				if cloudQuery != "" {
					query = cloudQuery
				} else {
					query = cloudText
				}
				query = NormalizeSmartHomeQuery(query)
			}
		}
	}

	if query != "" {
		s.executeSmartHomeAction(query, tNow)
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
