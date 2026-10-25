package web

import (
	"strings"
	"testing"
)

func TestGetIndexHTML(t *testing.T) {
	html := GetIndexHTML()
	if len(html) == 0 {
		t.Fatalf("Expected non-empty HTML")
	}
	content := string(html)
	if !strings.Contains(content, "Home Sentinel") {
		t.Errorf("Expected HTML to contain 'Home Sentinel'")
	}
}
