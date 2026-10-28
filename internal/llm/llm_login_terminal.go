package llm

import (
	"io"
	"strings"
)

type loginTerminalWriter struct {
	session  *loginSession
	terminal io.Writer
	pending  string
	trusted  bool
}

func (w *loginTerminalWriter) Write(data []byte) (int, error) {
	_, _ = w.session.Write(data)
	w.pending += string(data)
	replies := map[string]string{
		"\x1b[>c":   "\x1b[>0;276;0c",
		"\x1b[c":    "\x1b[?1;2c",
		"\x1b[6n":   "\x1b[1;1R",
		"\x1b[>q":   "\x1bP>|Sentinel(1.0)\x1b\\",
		"\x1b]11;?": "\x1b]11;rgb:0909/0a0a/0f0f\x1b\\",
		"\x1b]10;?": "\x1b]10;rgb:eeee/eeee/eeee\x1b\\",
	}
	for query, reply := range replies {
		if strings.Contains(w.pending, query) {
			if _, err := io.WriteString(w.terminal, reply); err != nil {
				return 0, err
			}
			w.pending = strings.ReplaceAll(w.pending, query, "")
		}
	}
	if !w.trusted && strings.Contains(w.pending, "Do you trust the contents of this project?") {
		if _, err := io.WriteString(w.terminal, "\r"); err != nil {
			return 0, err
		}
		w.trusted = true
	}
	if len(w.pending) > 512 {
		w.pending = w.pending[len(w.pending)-512:]
	}
	return len(data), nil
}
