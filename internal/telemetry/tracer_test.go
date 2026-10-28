package telemetry

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestRequestTraceFlow(t *testing.T) {
	trace := NewRequestTrace("req-test-123", "sess-test-456")

	trace.Record("understanding", 15*time.Millisecond, "ok", "Say Hi -> C2")
	trace.Record("resolution", 22*time.Millisecond, "ok", "Resolved Plex")
	trace.Record("execution", 45*time.Millisecond, "ok", "Opened Plex")
	total := trace.Finish()

	if total < 0 {
		t.Fatalf("Total time cannot be negative: %d", total)
	}

	var buf bytes.Buffer
	if err := trace.LogStructured(&buf); err != nil {
		t.Fatalf("Failed to log structured trace: %v", err)
	}

	var parsed RequestTrace
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Failed to unmarshal trace JSON: %v", err)
	}

	if parsed.RequestID != "req-test-123" {
		t.Errorf("Expected req-test-123, got %s", parsed.RequestID)
	}
	if len(parsed.Records) != 3 {
		t.Fatalf("Expected 3 records, got %d", len(parsed.Records))
	}
	if parsed.Records[0].Layer != "understanding" {
		t.Errorf("Expected first layer understanding, got %s", parsed.Records[0].Layer)
	}
}

func TestRedactInProduction(t *testing.T) {
	_ = os.Setenv("AGENT_ENV", "production")
	defer func() { _ = os.Unsetenv("AGENT_ENV") }()

	trace := NewRequestTrace("req-prod", "sess-prod")
	trace.Record("voice", 5*time.Millisecond, "ok", "người dùng nói mật khẩu")
	trace.Finish()

	if trace.Records[0].Detail != "[redacted_in_prod]" {
		t.Errorf("Expected redacted voice in prod, got %s", trace.Records[0].Detail)
	}
}
