package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/llm"
)

func captureLoginBrowser() bool {
	path := os.Getenv("SENTINEL_LOGIN_BROWSER_FILE")
	if path == "" {
		return false
	}
	if filepath.Base(path) != "browser-url" ||
		!strings.HasPrefix(filepath.Base(filepath.Dir(path)), "sentinel-login-") {
		return true
	}
	if authURL := llm.AuthorizationURL(strings.Join(os.Args[1:], " ")); authURL != "" {
		_ = os.WriteFile(path, []byte(authURL), 0600)
	}
	return true
}
