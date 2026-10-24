package main

import (
	"errors"
	"strconv"
	"strings"
)

const (
	defaultPort    = 8795
	defaultDataDir = "."
)

type Config struct {
	Port                   int
	HomeAssistantURL       string
	HomeAssistantPublicURL string
	GatewayURL             string
	DataDir                string
}

func LoadConfig(getenv func(string) string) (Config, error) {
	homeAssistantURL := strings.TrimRight(getenv("HA_URL"), "/")
	if homeAssistantURL == "" {
		return Config{}, errors.New("HA_URL is required, e.g. http://nas.local:8123")
	}

	port := defaultPort
	if raw := getenv("PORT"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 || parsed > 65535 {
			return Config{}, errors.New("PORT must be between 1 and 65535")
		}
		port = parsed
	}

	publicURL := strings.TrimRight(getenv("HA_PUBLIC_URL"), "/")
	if publicURL == "" {
		publicURL = homeAssistantURL
	}

	dataDir := getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = defaultDataDir
	}

	return Config{
		Port:                   port,
		HomeAssistantURL:       homeAssistantURL,
		HomeAssistantPublicURL: publicURL,
		GatewayURL:             strings.TrimRight(getenv("GATEWAY_URL"), "/"),
		DataDir:                dataDir,
	}, nil
}
