package speech

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

// TranscribeLocalPCM gửi trực tiếp dữ liệu PCM sang Wyoming Faster-Whisper :10300
func TranscribeLocalPCM(pcm []byte, modelName string) (string, error) {
	if modelName == "" {
		modelName = "base-int8"
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:10300", 2*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return "", err
	}

	tData, _ := json.Marshal(map[string]interface{}{"name": modelName, "language": "vi"})
	tHeader, _ := json.Marshal(map[string]interface{}{"type": "transcribe", "version": "1.10.2", "data_length": len(tData)})
	conn.Write(append(tHeader, '\n'))
	conn.Write(tData)

	sData, _ := json.Marshal(map[string]interface{}{"rate": 16000, "width": 2, "channels": 1})
	sHeader, _ := json.Marshal(map[string]interface{}{"type": "audio-start", "version": "1.10.2", "data_length": len(sData)})
	conn.Write(append(sHeader, '\n'))
	conn.Write(sData)

	chunkSize := 2048
	for i := 0; i < len(pcm); i += chunkSize {
		end := i + chunkSize
		if end > len(pcm) {
			end = len(pcm)
		}
		cData, _ := json.Marshal(map[string]interface{}{"rate": 16000, "width": 2, "channels": 1})
		cHeader, _ := json.Marshal(map[string]interface{}{
			"type":           "audio-chunk",
			"version":        "1.10.2",
			"data_length":    len(cData),
			"payload_length": end - i,
		})
		conn.Write(append(cHeader, '\n'))
		conn.Write(cData)
		conn.Write(pcm[i:end])
	}

	stopHeader, _ := json.Marshal(map[string]interface{}{"type": "audio-stop", "version": "1.10.2"})
	conn.Write(append(stopHeader, '\n'))

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return "", fmt.Errorf("read Whisper response: %w", err)
		}
		var h struct {
			Type          string `json:"type"`
			DataLength    int    `json:"data_length,omitempty"`
			PayloadLength int    `json:"payload_length,omitempty"`
		}
		_ = json.Unmarshal(line, &h)

		var dataBytes []byte
		if h.DataLength > 0 {
			dataBytes = make([]byte, h.DataLength)
			_, _ = io.ReadFull(reader, dataBytes)
		}
		if h.PayloadLength > 0 {
			pBytes := make([]byte, h.PayloadLength)
			_, _ = io.ReadFull(reader, pBytes)
		}

		if h.Type == "transcript" {
			var res struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(dataBytes, &res); err != nil {
				return "", err
			}
			return res.Text, nil
		}
	}
}
