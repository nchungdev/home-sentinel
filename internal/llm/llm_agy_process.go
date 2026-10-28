package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"
)

const agyCloseGrace = 10 * time.Second

type agyProcess struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	results       chan agyResult
	exited        chan struct{}
	turns         int
	inputTokens   int
	contextTokens int
}

type agyInput struct {
	Event   string          `json:"event"`
	Message agyInputMessage `json:"message"`
}

type agyInputMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (m *Manager) startAgyProcess() (*agyProcess, error) {
	if m.config.AgyPath == "" {
		return nil, fmt.Errorf("binary path empty")
	}
	cmd := exec.Command(m.config.AgyPath,
		"--input-format", "stream-json", "--output-format", "stream-json",
		"--disable-slash-commands", "-p", "")
	m.prepareCLI(cmd, m.config.AgyPath)
	if err := m.isolateAgy(cmd); err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	process := &agyProcess{
		cmd: cmd, stdin: stdin,
		results: make(chan agyResult, 1), exited: make(chan struct{}),
	}
	go process.read(stdout)
	return process, nil
}

func (p *agyProcess) read(stdout io.Reader) {
	defer close(p.exited)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	for scanner.Scan() {
		var event agyEvent
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Event == "result" {
			p.results <- event.Result
		}
	}
	_, _ = io.Copy(io.Discard, stdout)
	_ = p.cmd.Wait()
}

func (p *agyProcess) ask(ctx context.Context, text string) (agyResult, error) {
	line, err := json.Marshal(agyInput{
		Event: "user", Message: agyInputMessage{Role: "user", Content: text},
	})
	if err != nil {
		return agyResult{}, err
	}
	if _, err := p.stdin.Write(append(line, '\n')); err != nil {
		return agyResult{}, fmt.Errorf("agy daemon is not accepting input: %w", err)
	}

	select {
	case result := <-p.results:
		p.turns++
		p.contextTokens = result.Usage.InputTokens - p.inputTokens
		p.inputTokens = result.Usage.InputTokens
		if result.Status != "SUCCESS" || result.Response == "" {
			return result, fmt.Errorf("agy result %s", result.Status)
		}
		return result, nil
	case <-p.exited:
		return agyResult{}, fmt.Errorf("agy daemon exited")
	case <-ctx.Done():
		return agyResult{}, ctx.Err()
	}
}

func (p *agyProcess) alive() bool {
	select {
	case <-p.exited:
		return false
	default:
		return true
	}
}

func (p *agyProcess) close() {
	_ = p.stdin.Close()
	go func() {
		select {
		case <-p.exited:
		case <-time.After(agyCloseGrace):
			_ = p.cmd.Process.Kill()
		}
	}()
}
