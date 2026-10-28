package speech

import (
	"strings"
)

type Transcriber struct {
	groqClient *GroqClient
	localModel string
}

func NewTranscriber(groqKey string) *Transcriber {
	return &Transcriber{
		groqClient: NewGroqClient(groqKey),
		localModel: "base-int8",
	}
}

func (t *Transcriber) HasGroqKey() bool {
	return t.groqClient != nil && t.groqClient.APIKey != ""
}

func (t *Transcriber) UpdateGroqKey(key string) {
	if t.groqClient != nil {
		t.groqClient.APIKey = key
	}
}

// Transcribe ưu tiên Groq Cloud Whisper Large-v3 nếu forceCloud=true, ngược lại mặc định dùng Local Wyoming Whisper (tiết kiệm quota)
func (t *Transcriber) Transcribe(pcm []byte) (string, string, error) {
	return t.TranscribeLocal(pcm)
}

// TranscribeLocal giải mã bằng mô hình tiny trên NAS, hoàn toàn miễn phí 24/7 để quét wake-word
func (t *Transcriber) TranscribeLocal(pcm []byte) (string, string, error) {
	text, err := TranscribeLocalPCM(pcm, t.localModel)
	if err == nil && strings.TrimSpace(text) != "" {
		return text, "local_wyoming_whisper", nil
	}
	// Nếu local whisper chưa bật hoặc lỗi, và có Groq key thì fallback
	if t.HasGroqKey() {
		return t.TranscribeCloud(pcm)
	}
	return text, "local_wyoming_whisper", err
}

// TranscribeCloud dùng Groq Cloud Whisper Large-v3 siêu tốc, chỉ gọi khi cần nhận diện câu lệnh chính xác
func (t *Transcriber) TranscribeCloud(pcm []byte) (string, string, error) {
	if t.groqClient != nil && t.groqClient.APIKey != "" {
		text, err := t.groqClient.TranscribePCM(pcm)
		if err == nil && strings.TrimSpace(text) != "" {
			return text, "groq_whisper_large_v3", nil
		}
	}
	text, err := TranscribeLocalPCM(pcm, t.localModel)
	return text, "local_wyoming_whisper", err
}
