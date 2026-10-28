package nas_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/adapters/nas"
	"github.com/nchungdev/home-sentinel/internal/adapters/nas/nastest"
	"github.com/nchungdev/home-sentinel/internal/contracts"
)

func executeTarget(t *testing.T, fake *nastest.FakeNAS, target string) *contracts.ExecutionResult {
	t.Helper()
	adapter := nas.NewAdapter(fake.Start(t))
	step := &contracts.ActionStep{StepID: "step-1", ActionType: "system.status", TargetID: target}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("execute %s: %v", target, err)
	}
	if result.Detail == nil {
		t.Fatalf("execute %s: missing detail", target)
	}
	return result
}

func assertContainsAll(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("expected %q in report:\n%s", part, text)
		}
	}
}

func TestSnapshotReadsFakeNAS(t *testing.T) {
	adapter := nas.NewAdapter(nastest.DefaultNAS().Start(t))
	snapshot := adapter.Snapshot(context.Background())

	if snapshot.Host.Hostname != "omv-nas" || !snapshot.Host.HasLoad {
		t.Fatalf("unexpected host: %+v", snapshot.Host)
	}
	if snapshot.Host.Load1 != 0.42 {
		t.Errorf("expected load 0.42, got %v", snapshot.Host.Load1)
	}
	if len(snapshot.Disks) != 2 || snapshot.Disks[0].Mount != "/srv/media" {
		t.Fatalf("unexpected disks: %+v", snapshot.Disks)
	}
	if len(snapshot.Containers) != 3 || snapshot.Containers[0].Name != "plex" {
		t.Fatalf("unexpected containers: %+v", snapshot.Containers)
	}
	if snapshot.HostError+snapshot.DiskError+snapshot.DockerError != "" {
		t.Fatalf("unexpected errors: %+v", snapshot)
	}
}

func TestDiskReportShowsFreeSpaceAndWarning(t *testing.T) {
	result := executeTarget(t, nastest.DefaultNAS(), nas.TargetDisk)

	if result.Status != contracts.ActionStatusSucceeded {
		t.Fatalf("expected succeeded, got %s", result.Status)
	}
	assertContainsAll(t, *result.Detail,
		"Ổ /srv/media: còn trống 2,0 TB trên 8,0 TB (đã dùng 75%).",
		"Ổ /srv/backup: còn trống 256,0 GB trên 4,0 TB (đã dùng 94%). Ổ này sắp đầy.",
	)
	if strings.Contains(*result.Detail, "Docker") {
		t.Errorf("disk report should not mention docker:\n%s", *result.Detail)
	}
}

func TestDockerReportListsStoppedContainers(t *testing.T) {
	result := executeTarget(t, nastest.DefaultNAS(), nas.TargetDocker)

	assertContainsAll(t, *result.Detail, "Docker: 2/3 container đang chạy. Đang dừng: radarr.")
}

func TestFullReportCoversHostDisksAndDocker(t *testing.T) {
	result := executeTarget(t, nastest.DefaultNAS(), "nas")

	assertContainsAll(t, *result.Detail,
		"Máy omv-nas đang hoạt động, tải hệ thống 1 phút: 0,42.",
		"Ổ /srv/media",
		"Docker: 2/3",
	)
}

func TestDockerDownFailsDockerTargetOnly(t *testing.T) {
	fake := nastest.DefaultNAS()
	fake.DockerDown = true

	dockerResult := executeTarget(t, fake, nas.TargetDocker)
	if dockerResult.Status != contracts.ActionStatusFailed {
		t.Fatalf("expected failed docker report, got %s", dockerResult.Status)
	}
	assertContainsAll(t, *dockerResult.Detail, "Không kết nối được Docker")

	fullResult := executeTarget(t, fake, "nas")
	if fullResult.Status != contracts.ActionStatusSucceeded {
		t.Fatalf("partial failure should still succeed, got %s", fullResult.Status)
	}
	assertContainsAll(t, *fullResult.Detail, "Ổ /srv/media", "Không kết nối được Docker")
}

func TestMissingVolumeReportsDiskError(t *testing.T) {
	fake := nastest.DefaultNAS()
	probe := fake.Start(t)
	probe.Mounts = append(probe.Mounts, "/srv/missing")

	result, err := nas.NewAdapter(probe).Execute(
		context.Background(),
		&contracts.ActionStep{StepID: "step-1", TargetID: nas.TargetDisk},
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Status != contracts.ActionStatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	assertContainsAll(t, *result.Detail, "Không đọc được dung lượng ổ đĩa.")
	if strings.Contains(*result.Detail, "statfs") {
		t.Errorf("raw error should stay out of speech:\n%s", *result.Detail)
	}
}

func TestEmptyDockerAndNoLoadFile(t *testing.T) {
	fake := nastest.DefaultNAS()
	fake.Containers = nil
	probe := fake.Start(t)
	probe.LoadAvgPath = "/nonexistent/loadavg"

	result, err := nas.NewAdapter(probe).Execute(
		context.Background(),
		&contracts.ActionStep{StepID: "step-1", TargetID: "system"},
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertContainsAll(t, *result.Detail,
		"Máy omv-nas đang hoạt động.",
		"Docker không có container nào.",
	)
}

func TestNilStepIsRejected(t *testing.T) {
	adapter := nas.NewAdapter(nastest.DefaultNAS().Start(t))
	if _, err := adapter.Execute(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil step")
	}
}
