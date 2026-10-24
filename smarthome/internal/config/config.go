package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	GatewayURL string `json:"gateway_url"`
	Token      string `json:"token"`
	ListenPort int    `json:"listen_port"`
}

type capturedTokenPayload struct {
	Token string `json:"token"`
}

func Default() Config {
	return Config{
		GatewayURL: "https://192.168.1.2:8123",
		Token:      resolveToken(),
		ListenPort: 8791,
	}
}

func resolveToken() string {
	fromEnvironment := os.Getenv("GATEWAY_TOKEN")
	if fromEnvironment != "" {
		return fromEnvironment
	}

	searchPaths := []string{
		"local/dqsmart/runtime/captured_token.json",
		"../../local/dqsmart/runtime/captured_token.json",
	}

	for _, candidatePath := range searchPaths {
		absolutePath, errorClean := filepath.Abs(candidatePath)
		if errorClean != nil {
			continue
		}

		rawBytes, errorRead := os.ReadFile(absolutePath)
		if errorRead != nil {
			continue
		}

		var payload capturedTokenPayload
		if errorUnmarshal := json.Unmarshal(rawBytes, &payload); errorUnmarshal == nil {
			if payload.Token != "" {
				return payload.Token
			}
		}
	}

	return ""
}
