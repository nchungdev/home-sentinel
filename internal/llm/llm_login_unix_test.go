//go:build unix

package llm

import (
	"context"
	"os/exec"
	"testing"
)

func TestInteractiveLoginReceivesTerminalAndCapturesURL(t *testing.T) {
	command := exec.Command("/bin/sh", "-c",
		"stty raw -echo; printf '\\033[>c'; "+
			"dd bs=1 count=1 >/dev/null 2>&1; "+
			"printf '%s' 'https://auth.openai.com/codex/device'")
	session := &loginSession{state: LoginState{Running: true}}
	wait, err := startLoginCommand(context.Background(), command, session, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := wait(); err != nil {
		t.Fatal(err)
	}
	if session.snapshot().LoginURL != "https://auth.openai.com/codex/device" {
		t.Fatal("interactive login URL was not captured")
	}
}
