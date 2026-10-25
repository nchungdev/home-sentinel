package automation

import (
	"os"
	"os/exec"
	"strings"
)

func IsTVScreenOn(tvHost string) bool {
	if tvHost == "" {
		return false
	}
	adbBin := "/home/chungnh/bin/adb"
	if _, err := os.Stat(adbBin); err != nil {
		adbBin = "adb"
	}
	target := tvHost
	if !strings.Contains(target, ":") {
		target = target + ":5555"
	}

	cmd := exec.Command(adbBin, "-s", target, "shell", "dumpsys power | grep mWakefulness=")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "mWakefulness=Awake")
}
