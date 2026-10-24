package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	sessionsFileName  = "sessions.json"
	readHeaderTimeout = 5 * time.Second
)

func main() {
	config, configErr := LoadConfig(os.Getenv)
	if configErr != nil {
		fmt.Fprintln(os.Stderr, configErr)
		os.Exit(2)
	}

	homeAssistant := NewHomeAssistant(config.HomeAssistantURL, config.HomeAssistantPublicURL)
	sessions := NewSessionStore(filepath.Join(config.DataDir, sessionsFileName), time.Now)
	gateway := NewGateway(config, homeAssistant, sessions)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", config.Port),
		Handler:           gateway.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	fmt.Printf("FreeTV gateway on :%d\n", config.Port)
	fmt.Printf("Home Assistant at %s\n", config.HomeAssistantURL)
	if serveErr := server.ListenAndServe(); serveErr != nil {
		fmt.Fprintln(os.Stderr, serveErr)
		os.Exit(1)
	}
}
