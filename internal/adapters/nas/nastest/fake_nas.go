package nastest

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/adapters/nas"
)

const TiB = uint64(1) << 40

type Volume struct {
	Mount      string
	TotalBytes uint64
	FreeBytes  uint64
}

type Container struct {
	Name  string
	Image string
	State string
}

type FakeNAS struct {
	Hostname   string
	Load1      string
	Volumes    []Volume
	Containers []Container
	DockerDown bool
}

func DefaultNAS() *FakeNAS {
	return &FakeNAS{
		Hostname: "omv-nas",
		Load1:    "0.42",
		Volumes: []Volume{
			{Mount: "/srv/media", TotalBytes: 8 * TiB, FreeBytes: 2 * TiB},
			{Mount: "/srv/backup", TotalBytes: 4 * TiB, FreeBytes: TiB / 4},
		},
		Containers: []Container{
			{Name: "plex", Image: "plexinc/pms-docker", State: "running"},
			{Name: "sonarr", Image: "linuxserver/sonarr", State: "running"},
			{Name: "radarr", Image: "linuxserver/radarr", State: "exited"},
		},
	}
}

func (f *FakeNAS) Start(t testing.TB) *nas.SystemProbe {
	t.Helper()

	dir, err := os.MkdirTemp("", "nas")
	if err != nil {
		t.Fatalf("create fake nas dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	loadAvgPath := filepath.Join(dir, "loadavg")
	loadLine := fmt.Sprintf("%s 0.30 0.25 1/123 4567\n", f.Load1)
	if err := os.WriteFile(loadAvgPath, []byte(loadLine), 0o600); err != nil {
		t.Fatalf("write fake loadavg: %v", err)
	}

	socketPath := filepath.Join(dir, "docker.sock")
	if !f.DockerDown {
		f.startDocker(t, socketPath)
	}

	return &nas.SystemProbe{
		Mounts:       f.mounts(),
		DockerSocket: socketPath,
		LoadAvgPath:  loadAvgPath,
		StatFS:       f.statFS,
		Hostname:     func() (string, error) { return f.Hostname, nil },
	}
}

func (f *FakeNAS) startDocker(t testing.TB, socketPath string) {
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen fake docker socket: %v", err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(f.serveDocker))
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
}

func (f *FakeNAS) serveDocker(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/containers/json" {
		http.NotFound(w, r)
		return
	}

	rows := make([]map[string]any, 0, len(f.Containers))
	for _, container := range f.Containers {
		rows = append(rows, map[string]any{
			"Names": []string{"/" + container.Name},
			"Image": container.Image,
			"State": container.State,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

func (f *FakeNAS) mounts() []string {
	mounts := make([]string, 0, len(f.Volumes))
	for _, volume := range f.Volumes {
		mounts = append(mounts, volume.Mount)
	}
	return mounts
}

func (f *FakeNAS) statFS(path string) (uint64, uint64, error) {
	for _, volume := range f.Volumes {
		if volume.Mount == path {
			return volume.TotalBytes, volume.FreeBytes, nil
		}
	}
	return 0, 0, fmt.Errorf("no such volume: %s", path)
}
