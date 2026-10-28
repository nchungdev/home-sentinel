//go:build unix

package llm

import (
	"context"
	"io"
	"os/exec"
	"time"

	"github.com/creack/pty"
)

func startLoginCommand(
	_ context.Context, command *exec.Cmd, session *loginSession, interactive bool,
) (func() error, error) {
	if !interactive {
		return startPipedLogin(command, session)
	}
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 32, Cols: 120})
	if err != nil {
		return nil, err
	}
	session.input = terminal
	copied := make(chan struct{})
	go func() {
		defer close(copied)
		writer := &loginTerminalWriter{session: session, terminal: terminal}
		_, _ = io.Copy(writer, terminal)
	}()
	return func() error {
		err := command.Wait()
		select {
		case <-copied:
		case <-time.After(time.Second):
		}
		terminal.Close()
		<-copied
		return err
	}, nil
}
