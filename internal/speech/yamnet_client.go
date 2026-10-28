package speech

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"time"
)

type YAMNetPrediction struct {
	ClassID int     `json:"class_id"`
	Label   string  `json:"label"`
	Score   float64 `json:"score"`
}

type YAMNetResponse struct {
	TopLabel    string             `json:"top_label"`
	TopScore    float64            `json:"top_score"`
	IsSpeech    bool               `json:"is_speech"`
	IsAlarm     bool               `json:"is_alarm"`
	IsClap      bool               `json:"is_clap"`
	Predictions []YAMNetPrediction `json:"predictions"`
	Error       string             `json:"error,omitempty"`
}

// ClassifyYAMNet gửi PCM 16kHz mono sang dịch vụ YAMNet ONNX cục bộ :10400
func ClassifyYAMNet(pcm []byte) (*YAMNetResponse, error) {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:10400", 500*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(1 * time.Second))

	// Header: 4 bytes độ dài (big-endian)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(pcm)))
	if _, err := conn.Write(lenBuf); err != nil {
		return nil, err
	}
	if _, err := conn.Write(pcm); err != nil {
		return nil, err
	}

	respBytes, err := io.ReadAll(conn)
	if err != nil {
		return nil, err
	}

	var res YAMNetResponse
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
