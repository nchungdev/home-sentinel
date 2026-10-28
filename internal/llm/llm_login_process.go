package llm

import "os/exec"

func startPipedLogin(command *exec.Cmd, session *loginSession) (func() error, error) {
	command.Stdout, command.Stderr = session, session
	input, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	session.input = input
	if err := command.Start(); err != nil {
		input.Close()
		return nil, err
	}
	return command.Wait, nil
}
