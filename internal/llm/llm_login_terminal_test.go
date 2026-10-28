package llm

import (
	"bytes"
	"strings"
	"testing"
)

func TestTerminalRepliesToFragmentedQueriesAndTrustsOnlyOnce(t *testing.T) {
	var replies bytes.Buffer
	session := &loginSession{}
	writer := &loginTerminalWriter{session: session, terminal: &replies}
	writer.Write([]byte("\x1b[>"))
	writer.Write([]byte("c\x1b]11;?\a"))
	if !strings.Contains(replies.String(), "\x1b[>0;276;0c") ||
		!strings.Contains(replies.String(), "rgb:0909/0a0a/0f0f") {
		t.Fatal("terminal query not answered")
	}
	question := []byte("Do you trust the contents of this project?")
	writer.Write(question)
	before := replies.Len()
	writer.Write(question)
	if replies.Len() != before {
		t.Fatal("workspace trust submitted twice")
	}
}
