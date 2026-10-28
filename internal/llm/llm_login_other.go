//go:build !unix && !windows

package llm

import (
	"context"
	"os/exec"
)

func startLoginCommand(
	_ context.Context, command *exec.Cmd, session *loginSession, interactive bool,
) (func() error, error) {
	return startPipedLogin(command, session)
}
