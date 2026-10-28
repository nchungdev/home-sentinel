package nas

import "context"

type Disk struct {
	Mount      string `json:"mount"`
	TotalBytes uint64 `json:"total_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
}

func (d Disk) UsedPercent() float64 {
	if d.TotalBytes == 0 {
		return 0
	}
	return float64(d.TotalBytes-d.FreeBytes) * 100 / float64(d.TotalBytes)
}

type Container struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Image string `json:"image"`
}

func (c Container) Running() bool {
	return c.State == "running"
}

type Host struct {
	Hostname string  `json:"hostname"`
	Load1    float64 `json:"load_1m"`
	HasLoad  bool    `json:"has_load"`
}

type Snapshot struct {
	Host        Host        `json:"host"`
	Disks       []Disk      `json:"disks"`
	Containers  []Container `json:"containers"`
	HostError   string      `json:"host_error,omitempty"`
	DiskError   string      `json:"disk_error,omitempty"`
	DockerError string      `json:"docker_error,omitempty"`
}

type Probe interface {
	Host(ctx context.Context) (Host, error)
	Disks(ctx context.Context) ([]Disk, error)
	Containers(ctx context.Context) ([]Container, error)
}

func Collect(ctx context.Context, probe Probe) Snapshot {
	var snapshot Snapshot

	host, err := probe.Host(ctx)
	snapshot.Host = host
	snapshot.HostError = errorText(err)

	disks, err := probe.Disks(ctx)
	snapshot.Disks = disks
	snapshot.DiskError = errorText(err)

	containers, err := probe.Containers(ctx)
	snapshot.Containers = containers
	snapshot.DockerError = errorText(err)

	return snapshot
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
