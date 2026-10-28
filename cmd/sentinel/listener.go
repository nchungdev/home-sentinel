package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"time"

	"github.com/nchungdev/home-sentinel/internal/speech"
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
									server.StateMutex.RLock()
									userOnSofa := server.UserOnSofa
									server.StateMutex.RUnlock()
									if userOnSofa {
										server.AutomationMgr.RecordActivity("tiếng vỗ tay trên sofa")
										server.AutomationMgr.HandleClapOnSofa(userOnSofa)
									} else {
										server.AutomationMgr.HandleAudioActivity("tiếng vỗ tay khi chưa thấy người")
									}
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

							// Phối hợp Camera + Âm thanh:
							// - Nếu phòng đã có người (Camera xác nhận trước đó) -> gia hạn bộ đếm tắt đèn.
							// - Nếu phòng đang trống -> xoay Camera về phòng khách kiểm tra xem có người thật không (không tự bật đèn/giữ đèn nhầm).
							if server.AutomationMgr != nil {
								server.AutomationMgr.HandleAudioActivity("âm thanh: " + audioFeatures.Description)
							}
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
