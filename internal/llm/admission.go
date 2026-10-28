package llm

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

type SystemMetricsProvider interface {
	GetCPULoadPercent() (float64, error)
	GetAvailableRAMMB() (uint64, error)
	GetActivePlexStreams() (int, error)
}

type AdmissionConfig struct {
	MaxCPULoadPercent    float64
	MinAvailableRAMMB    uint64
	MaxActivePlexStreams int
	MaxLocalConcurrency  int
}

type AdmissionController struct {
	mu              sync.Mutex
	config          AdmissionConfig
	metricsProvider SystemMetricsProvider
	semaphore       chan struct{}
}

func DefaultAdmissionConfig() AdmissionConfig {
	return AdmissionConfig{
		MaxCPULoadPercent:    85.0,
		MinAvailableRAMMB:    1024,
		MaxActivePlexStreams: 0,
		MaxLocalConcurrency:  1,
	}
}

func NewAdmissionController(
	cfg AdmissionConfig,
	provider SystemMetricsProvider,
) *AdmissionController {
	if cfg.MaxCPULoadPercent <= 0 {
		cfg.MaxCPULoadPercent = 85.0
	}
	if cfg.MinAvailableRAMMB <= 0 {
		cfg.MinAvailableRAMMB = 1024
	}
	if cfg.MaxLocalConcurrency <= 0 {
		cfg.MaxLocalConcurrency = 1
	}

	return &AdmissionController{
		config:          cfg,
		metricsProvider: provider,
		semaphore:       make(chan struct{}, cfg.MaxLocalConcurrency),
	}
}

func (a *AdmissionController) AcquireLocalInference(
	ctx context.Context,
) (bool, string, func()) {
	if a.metricsProvider != nil {
		cpuLoad, err := a.metricsProvider.GetCPULoadPercent()
		if err == nil && cpuLoad > a.config.MaxCPULoadPercent {
			return false, fmt.Sprintf(
				"CPU load %.1f%% exceeds limit of %.1f%%",
				cpuLoad,
				a.config.MaxCPULoadPercent,
			), nil
		}

		ramMB, err := a.metricsProvider.GetAvailableRAMMB()
		if err == nil && ramMB < a.config.MinAvailableRAMMB {
			return false, fmt.Sprintf(
				"Available RAM %d MB is below minimum %d MB",
				ramMB,
				a.config.MinAvailableRAMMB,
			), nil
		}

		plexStreams, err := a.metricsProvider.GetActivePlexStreams()
		if err == nil && plexStreams > a.config.MaxActivePlexStreams {
			return false, fmt.Sprintf(
				"Plex has %d active streaming sessions",
				plexStreams,
			), nil
		}
	}

	select {
	case a.semaphore <- struct{}{}:
		releaseFunc := func() {
			select {
			case <-a.semaphore:
			default:
			}
		}
		return true, "Admitted", releaseFunc
	default:
		return false, "Local inference concurrency limit reached", nil
	}
}

type DefaultSystemMetricsProvider struct{}

func (d *DefaultSystemMetricsProvider) GetCPULoadPercent() (float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0.0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0.0, fmt.Errorf("invalid loadavg format")
	}
	oneMinLoad, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0.0, err
	}
	// Intel N150 has 4 cores
	cpuPercent := (oneMinLoad / 4.0) * 100.0
	return cpuPercent, nil
}

func (d *DefaultSystemMetricsProvider) GetAvailableRAMMB() (uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return kb / 1024, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("MemAvailable not found")
}

func (d *DefaultSystemMetricsProvider) GetActivePlexStreams() (int, error) {
	return 0, nil
}
