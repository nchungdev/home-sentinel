package speech

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

type GroqClient struct {
	APIKey     string
	HTTPClient *http.Client
}

func NewGroqClient(apiKey string) *GroqClient {
	if apiKey == "" {
		apiKey = os.Getenv("GROQ_API_KEY")
	}
	return &GroqClient{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// TranscribePCM gửi PCM 16kHz mono sang Groq Whisper Large-v3
func (c *GroqClient) TranscribePCM(pcm []byte) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("chưa cấu hình GROQ_API_KEY")
	}

	wavBytes := pcmToWav(pcm, 16000, 1)

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	part, err := writer.CreateFormFile("file", "speech.wav")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(wavBytes); err != nil {
		return "", err
	}

	_ = writer.WriteField("model", "whisper-large-v3")
	_ = writer.WriteField("language", "vi")
	_ = writer.WriteField("response_format", "json")
	_ = writer.WriteField("temperature", "0.0")

	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/audio/transcriptions", &requestBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Groq API error (%d): %s", resp.StatusCode, string(respBytes))
	}

	var res struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	return strings.TrimSpace(res.Text), nil
}

func pcmToWav(pcm []byte, sampleRate int, numChannels int) []byte {
	bitsPerSample := 16
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := len(pcm)
	chunkSize := 36 + dataSize

	buf := new(bytes.Buffer)
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(chunkSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(numChannels))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	buf.Write(pcm)

	return buf.Bytes()
}
