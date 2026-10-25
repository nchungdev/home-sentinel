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
		localModel: "tiny-int8",
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

// Transcribe ưu tiên Groq Cloud Whisper Large-v3 siêu tốc, nếu lỗi hoặc chưa có key sẽ fallback sang Wyoming Local
func (t *Transcriber) Transcribe(pcm []byte) (string, string, error) {
	if t.groqClient != nil && t.groqClient.APIKey != "" {
		text, err := t.groqClient.TranscribePCM(pcm)
		if err == nil && strings.TrimSpace(text) != "" {
			return text, "groq_whisper_large_v3", nil
		}
	}

	text, err := TranscribeLocalPCM(pcm, t.localModel)
	return text, "local_wyoming_whisper", err
}
