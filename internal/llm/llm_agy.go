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

const agyExitGrace = 30 * time.Second

type agyResult struct {
	Status   string   `json:"status"`
	Response string   `json:"response"`
	Usage    agyUsage `json:"usage"`
}

type agyUsage struct {
	InputTokens int `json:"input_tokens"`
}

type agyEvent struct {
	Event  string    `json:"event"`
	Result agyResult `json:"result"`
}

func (m *Manager) callAgy(ctx context.Context, prompt string, onChunk func(string)) error {
	if m.config.AgyPath == "" {
		return fmt.Errorf("binary path empty")
	}
	cmd := exec.Command(m.config.AgyPath,
		"--output-format", "stream-json", "--disable-slash-commands", "-p", prompt)
	m.prepareCLI(cmd, m.config.AgyPath)
	if err := m.isolateAgy(cmd); err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	answered := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			_ = stdout.Close()
		case <-answered:
		}
	}()

	result, found := readAgyResult(stdout)
	close(answered)
	go finishAgy(cmd, stdout)

	if err := ctx.Err(); err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("agy exited without a result")
	}
	if result.Status != "SUCCESS" || result.Response == "" {
		return fmt.Errorf("agy result %s", result.Status)
	}
	onChunk(result.Response)
	return nil
}

func readAgyResult(stdout io.Reader) (agyResult, bool) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	for scanner.Scan() {
		var event agyEvent
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Event != "result" {
			continue
		}
		return event.Result, true
	}
	return agyResult{}, false
}

func finishAgy(cmd *exec.Cmd, stdout io.Reader) {
	killer := time.AfterFunc(agyExitGrace, func() { _ = cmd.Process.Kill() })
	defer killer.Stop()
	_, _ = io.Copy(io.Discard, stdout)
	_ = cmd.Wait()
}
