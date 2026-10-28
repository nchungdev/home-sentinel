package nas

import (
	"fmt"
	"strings"
)

const (
	diskWarnPercent = 90.0

	textDiskUnavailable   = "Không đọc được dung lượng ổ đĩa."
	textDockerUnavailable = "Không kết nối được Docker."
	textNoDisks           = "Chưa cấu hình ổ đĩa nào để theo dõi."
	textNoContainers      = "Docker không có container nào."
)

func renderHost(snapshot Snapshot) (string, bool) {
	if snapshot.HostError != "" {
		return "", false
	}

	name := snapshot.Host.Hostname
	if name == "" {
		name = "NAS"
	}
	if !snapshot.Host.HasLoad {
		return fmt.Sprintf("Máy %s đang hoạt động.", name), true
	}
	return fmt.Sprintf(
		"Máy %s đang hoạt động, tải hệ thống 1 phút: %s.",
		name, decimal(snapshot.Host.Load1, 2),
	), true
}

func renderDisks(snapshot Snapshot) (string, bool) {
	if snapshot.DiskError != "" {
		return textDiskUnavailable, false
	}
	if len(snapshot.Disks) == 0 {
		return textNoDisks, true
	}

	lines := make([]string, 0, len(snapshot.Disks))
	for _, disk := range snapshot.Disks {
		lines = append(lines, describeDisk(disk))
	}
	return strings.Join(lines, "\n"), true
}

func describeDisk(disk Disk) string {
	used := disk.UsedPercent()
	line := fmt.Sprintf(
		"Ổ %s: còn trống %s trên %s (đã dùng %s%%).",
		disk.Mount,
		humanBytes(disk.FreeBytes),
		humanBytes(disk.TotalBytes),
		decimal(used, 0),
	)
	if used >= diskWarnPercent {
		line += " Ổ này sắp đầy."
	}
	return line
}

func renderContainers(snapshot Snapshot) (string, bool) {
	if snapshot.DockerError != "" {
		return textDockerUnavailable, false
	}
	if len(snapshot.Containers) == 0 {
		return textNoContainers, true
	}

	var stopped []string
	running := 0
	for _, container := range snapshot.Containers {
		if container.Running() {
			running++
			continue
		}
		stopped = append(stopped, container.Name)
	}

	line := fmt.Sprintf(
		"Docker: %d/%d container đang chạy.", running, len(snapshot.Containers),
	)
	if len(stopped) > 0 {
		line += " Đang dừng: " + strings.Join(stopped, ", ") + "."
	}
	return line, true
}

func humanBytes(bytes uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	value := float64(bytes)
	unitIndex := 0
	for value >= 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}
	if unitIndex == 0 {
		return fmt.Sprintf("%d B", bytes)
	}
	return decimal(value, 1) + " " + units[unitIndex]
}

func decimal(value float64, precision int) string {
	return strings.Replace(fmt.Sprintf("%.*f", precision, value), ".", ",", 1)
}
