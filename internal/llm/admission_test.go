package llm

import (
	"context"
	"testing"
)

type mockMetricsProvider struct {
	cpuPercent  float64
	ramMB       uint64
	plexStreams int
}

func (m *mockMetricsProvider) GetCPULoadPercent() (float64, error) {
	return m.cpuPercent, nil
}

func (m *mockMetricsProvider) GetAvailableRAMMB() (uint64, error) {
	return m.ramMB, nil
}

func (m *mockMetricsProvider) GetActivePlexStreams() (int, error) {
	return m.plexStreams, nil
}

func TestAdmissionAdmitted(t *testing.T) {
	mock := &mockMetricsProvider{
		cpuPercent:  35.0,
		ramMB:       8192,
		plexStreams: 0,
	}
	cfg := DefaultAdmissionConfig()
	admission := NewAdmissionController(cfg, mock)

	admitted, reason, release := admission.AcquireLocalInference(context.Background())
	if !admitted {
		t.Fatalf("Expected admitted, got rejected: %s", reason)
	}
	if release == nil {
		t.Fatalf("Expected non-nil release func")
	}
	release()
}

func TestAdmissionRejectedHighCPU(t *testing.T) {
	mock := &mockMetricsProvider{
		cpuPercent:  90.5,
		ramMB:       8192,
		plexStreams: 0,
	}
	cfg := DefaultAdmissionConfig()
	admission := NewAdmissionController(cfg, mock)

	admitted, reason, release := admission.AcquireLocalInference(context.Background())
	if admitted {
		t.Fatalf("Expected rejected due to high CPU, got admitted")
	}
	if release != nil {
		t.Errorf("Expected nil release func on rejection")
	}
	if reason == "" {
		t.Errorf("Expected explanation reason")
	}
}

func TestAdmissionRejectedLowRAM(t *testing.T) {
	mock := &mockMetricsProvider{
		cpuPercent:  40.0,
		ramMB:       512,
		plexStreams: 0,
	}
	cfg := DefaultAdmissionConfig()
	admission := NewAdmissionController(cfg, mock)

	admitted, _, _ := admission.AcquireLocalInference(context.Background())
	if admitted {
		t.Fatalf("Expected rejected due to low RAM")
	}
}

func TestAdmissionRejectedPlexActive(t *testing.T) {
	mock := &mockMetricsProvider{
		cpuPercent:  40.0,
		ramMB:       8192,
		plexStreams: 2,
	}
	cfg := DefaultAdmissionConfig()
	admission := NewAdmissionController(cfg, mock)

	admitted, _, _ := admission.AcquireLocalInference(context.Background())
	if admitted {
		t.Fatalf("Expected rejected due to active Plex streams")
	}
}

func TestAdmissionConcurrencyLimit(t *testing.T) {
	mock := &mockMetricsProvider{
		cpuPercent:  30.0,
		ramMB:       8192,
		plexStreams: 0,
	}
	cfg := DefaultAdmissionConfig()
	cfg.MaxLocalConcurrency = 1
	admission := NewAdmissionController(cfg, mock)

	admitted1, _, release1 := admission.AcquireLocalInference(context.Background())
	if !admitted1 {
		t.Fatalf("First acquire should succeed")
	}

	admitted2, reason2, release2 := admission.AcquireLocalInference(context.Background())
	if admitted2 {
		t.Fatalf("Second concurrent acquire should be rejected")
	}
	if reason2 != "Local inference concurrency limit reached" {
		t.Errorf("Unexpected reason: %s", reason2)
	}
	if release2 != nil {
		t.Errorf("Expected nil release on reject")
	}

	release1()

	admitted3, _, release3 := admission.AcquireLocalInference(context.Background())
	if !admitted3 {
		t.Fatalf("Third acquire should succeed after release")
	}
	release3()
}
