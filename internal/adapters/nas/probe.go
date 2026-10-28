package nas

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultDockerSocket = "/var/run/docker.sock"
	DefaultLoadAvgPath  = "/proc/loadavg"
	dockerListURL       = "http://docker/containers/json?all=1"
	dockerTimeout       = 3 * time.Second
)

type StatFunc func(path string) (totalBytes uint64, freeBytes uint64, err error)

type SystemProbe struct {
	Mounts       []string
	DockerSocket string
	LoadAvgPath  string
	StatFS       StatFunc
	Hostname     func() (string, error)
}

func NewSystemProbeFromEnv() *SystemProbe {
	return &SystemProbe{
		Mounts:       parseMounts(os.Getenv("AGENT_NAS_MOUNTS")),
		DockerSocket: envOrDefault("AGENT_DOCKER_SOCKET", DefaultDockerSocket),
		LoadAvgPath:  DefaultLoadAvgPath,
		StatFS:       statFilesystem,
		Hostname:     os.Hostname,
	}
}

func (p *SystemProbe) Host(ctx context.Context) (Host, error) {
	var host Host
	if p.Hostname != nil {
		name, err := p.Hostname()
		if err != nil {
			return host, err
		}
		host.Hostname = name
	}

	raw, err := os.ReadFile(p.LoadAvgPath)
	if err != nil {
		return host, nil
	}

	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return host, nil
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return host, fmt.Errorf("parse %s: %w", p.LoadAvgPath, err)
	}
	host.Load1 = load
	host.HasLoad = true

	return host, nil
}

func (p *SystemProbe) Disks(ctx context.Context) ([]Disk, error) {
	disks := make([]Disk, 0, len(p.Mounts))
	for _, mount := range p.Mounts {
		total, free, err := p.StatFS(mount)
		if err != nil {
			return disks, fmt.Errorf("statfs %s: %w", mount, err)
		}
		disks = append(disks, Disk{Mount: mount, TotalBytes: total, FreeBytes: free})
	}
	return disks, nil
}

func (p *SystemProbe) Containers(ctx context.Context) ([]Container, error) {
	client := &http.Client{
		Timeout: dockerTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", p.DockerSocket)
			},
		},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, dockerListURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker api status %d", response.StatusCode)
	}

	var rows []struct {
		Names []string `json:"Names"`
		Image string   `json:"Image"`
		State string   `json:"State"`
	}
	if err := json.NewDecoder(response.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("decode docker containers: %w", err)
	}

	containers := make([]Container, 0, len(rows))
	for _, row := range rows {
		containers = append(containers, Container{
			Name:  containerName(row.Names),
			Image: row.Image,
			State: row.State,
		})
	}
	return containers, nil
}

func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

func parseMounts(rawList string) []string {
	var mounts []string
	for _, part := range strings.Split(rawList, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			mounts = append(mounts, trimmed)
		}
	}
	if len(mounts) == 0 {
		return []string{"/"}
	}
	return mounts
}

func envOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
