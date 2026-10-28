package llm

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/UserExistsError/conpty"
)

type windowsLoginTerminal struct {
	*conpty.ConPty
	once       sync.Once
	closeError error
}

func (t *windowsLoginTerminal) Close() error {
	t.once.Do(func() { t.closeError = t.ConPty.Close() })
	return t.closeError
}

func startLoginCommand(
	ctx context.Context, command *exec.Cmd, session *loginSession, interactive bool,
) (func() error, error) {
	if !interactive {
		return startPipedLogin(command, session)
	}
	arguments := make([]string, 0, len(command.Args))
	for _, argument := range command.Args {
		arguments = append(arguments, syscall.EscapeArg(argument))
	}
	console, err := conpty.Start(strings.Join(arguments, " "),
		conpty.ConPtyDimensions(120, 32), conpty.ConPtyWorkDir(command.Dir),
		conpty.ConPtyEnv(command.Env))
	if err != nil {
		return nil, err
	}
	terminal := &windowsLoginTerminal{ConPty: console}
	session.input = terminal
	copied := make(chan struct{})
	go func() {
		defer close(copied)
		writer := &loginTerminalWriter{session: session, terminal: terminal}
		_, _ = io.Copy(writer, terminal)
	}()
	return func() error {
		exitCode, err := console.Wait(ctx)
		terminal.Close()
		<-copied
		if err != nil {
			return err
		}
		if exitCode != 0 {
			return fmt.Errorf("CLI exit %d", exitCode)
		}
		return nil
	}, nil
}
