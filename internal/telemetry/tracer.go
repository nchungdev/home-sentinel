package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type LayerRecord struct {
	Layer     string `json:"layer"`
	ElapsedMs int64  `json:"elapsed_ms"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}

type RequestTrace struct {
	mu        sync.Mutex
	RequestID string         `json:"request_id"`
	SessionID string         `json:"session_id,omitempty"`
	StartTime time.Time      `json:"-"`
	TotalMs   int64          `json:"total_ms"`
	Records   []*LayerRecord `json:"records"`
}

func NewRequestTrace(requestID, sessionID string) *RequestTrace {
	return &RequestTrace{
		RequestID: requestID,
		SessionID: sessionID,
		StartTime: time.Now(),
		Records:   make([]*LayerRecord, 0, 6),
	}
}

func (t *RequestTrace) Record(layer string, elapsed time.Duration, status, detail string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	isProd := os.Getenv("AGENT_ENV") == "production" || os.Getenv("GO_ENV") == "production"
	if isProd && (layer == "input" || layer == "voice") {
		detail = "[redacted_in_prod]"
	}

	t.Records = append(t.Records, &LayerRecord{
		Layer:     layer,
		ElapsedMs: elapsed.Milliseconds(),
		Status:    status,
		Detail:    detail,
	})
}

func (t *RequestTrace) Finish() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.TotalMs = time.Since(t.StartTime).Milliseconds()
	return t.TotalMs
}

func (t *RequestTrace) ToJSON() ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return json.Marshal(t)
}

func (t *RequestTrace) LogStructured(w io.Writer) error {
	data, err := t.ToJSON()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}
