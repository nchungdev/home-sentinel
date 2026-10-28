//go:build unix

package llm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeCodexScript = `#!/bin/sh
for argument in "$@"; do
  if [ "$argument" = "--skip-git-repo-check" ]; then
    echo "OK"
    exit 0
  fi
done
echo "Not inside a trusted directory and --skip-git-repo-check was not specified." >&2
exit 1
`

func TestCodexRunsOutsideGitRepository(t *testing.T) {
	codexPath := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(codexPath, []byte(fakeCodexScript), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{config: &Config{SelectedEngine: "codex", CodexPath: codexPath}}

	var response strings.Builder
	err := manager.StreamChatContext(context.Background(), "hello", func(chunk string) {
		response.WriteString(chunk)
	})
	if err != nil {
		t.Fatalf("codex outside a git repository: %v", err)
	}
	if !strings.Contains(response.String(), "OK") {
		t.Fatalf("unexpected response %q", response.String())
	}
}
