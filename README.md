# Home Sentinel Server

Autonomous smart home backend and AI coordinator for Android TV, Tapo PTZ security cameras, and Home Assistant.

---

## Table of Contents

- [Overview](#overview)
- [System Architecture](#system-architecture)
- [Directory Layout](#directory-layout)
- [Agent Server Installation & Setup Guide](#agent-server-installation--setup-guide)
  - [1. Prerequisites](#1-prerequisites)
  - [2. Configuration Files](#2-configuration-files)
  - [3. Building the Binary](#3-building-the-binary)
  - [4. Deploying to Remote NAS / Server](#4-deploying-to-remote-nas--server)
  - [5. Running as a Daemon / Systemd Service](#5-running-as-a-daemon--systemd-service)
  - [6. Verification & Health Checklist](#6-verification--health-checklist)
- [Communication Protocols & Endpoints](#communication-protocols--endpoints)
- [Development Guidelines](#development-guidelines)

---

## Overview

Home Sentinel runs as a unified Go server (`:8788`) providing:

1. **AI Agent Pipeline (`/v2/ask`)**: Multi-tier engine understanding natural Vietnamese speech/text, orchestrating Home Assistant entities, and querying media servers.
2. **Android TV Coordinator**: Network ADB control (screen wakeup/sleep) and WebSocket (`/ws/duinch`) for live HUD notifications, proactive greeting banners, and expressions.
3. **Tapo PTZ Camera Integration**: Real-time microphone listening via RTSP PCM stream, two-way audio broadcast (`./bin/tapo_speaker`), and ONVIF PTZ preset control.
4. **HA-Style Automation Engine**: Authentic When (Triggers) / And If (Conditions) / Then Do (Actions Pipeline) rule engine stored in `configs/automation.json`.
5. **Zero-Dependency Web Dashboard (`web/`)**: Real-time control dashboard, entity manager, automation rule editor, camera PTZ controller, and live log terminal.

---

## System Architecture

```
[Voice / Audio / Web Chat / Automation Triggers]
                      │
                      ▼
 1. Transport Layer (Auth, Token, Body Size Limiter, Context, NDJSON Stream)
                      │
                      ▼
 2. Understanding Layer (Vietnamese diacritics, intent & slot extraction)
                      │
                      ▼
 3. Resolution Layer (HA fuzzy matching, Plex search, TV ADB mapper, ActionPlan)
                      │
                      ▼
 4. Arbitration Layer (Priority admission, conflict resolution, cooldowns)
                      │
                      ▼
 5. Execution Layer (Adapters: Home Assistant, Camera Tapo PTZ & Speaker, ADB)
                      │
                      ▼
 6. Presentation Layer (Duinch Bot mood/expression, TTS speech text, client actions)
                      │
                      ▼
[Output: NDJSON Stream / WebSocket / ADB / Camera Speaker / HA Service]
```

Detailed architectural specifications are documented in [docs/architecture.md](docs/architecture.md).

---

## Directory Layout

```
home-sentinel/
├── agent/
│   ├── cmd/agent_server/     # Main server entrypoint, HTTP/WS handlers, and routing
│   └── pkg/
│       ├── adapters/         # HA and Camera execution adapters
│       ├── arbitration/      # Conflict resolution and event arbitration
│       ├── automation/       # HA-style automation engine, TV ADB controls, config store
│       ├── camstream/        # Tapo camera RTSP stream and ONVIF session manager
│       ├── contracts/        # Shared data models (InputEvent, ActionPlan, etc.)
│       ├── haclient/         # Direct REST client for Home Assistant
│       ├── llm/              # Multi-engine LLM manager (Groq, OpenAI, Local Qwen)
│       ├── mediabridge/      # Plex, Radarr, Sonarr media integrations
│       ├── orchestrator/     # Core 6-layer pipeline coordinator
│       ├── proactive/        # Suggestion engine for TV idle screen
│       ├── resolution/       # Entity matching and ActionPlan generator
│       ├── speech/           # Audio transcription and TTS bindings
│       ├── telemetry/        # Metric tracking and latency monitoring
│       ├── transport/        # /v2/ask NDJSON streaming HTTP handler
│       └── understanding/    # Intent classification and phonetic normalization
├── configs/
│   └── automation.json       # Persistent automation rules
├── docs/
│   └── architecture.md       # Technical design and protocol documentation
└── web/                      # Embedded modern web dashboard (HTML, CSS, JS)
```

---

## Agent Server Installation & Setup Guide

> **Note for AI Agents**: Follow these sequential instructions when setting up, compiling, deploying, or troubleshooting the Home Sentinel server.

### 1. Prerequisites

Verify that the target machine (NAS / Linux server) has the following dependencies:
- **Architecture**: Linux `x86_64` (`amd64`) or `aarch64` (`arm64`).
- **FFmpeg**: Required for camera audio stream processing (`ffmpeg -version`).
- **ADB**: Required for Android TV control (`adb version` or `/home/chungnh/bin/adb`).
- **Network Access**:
  - Android TV: Port `5555` open on `192.168.1.8` (or target TV IP).
  - Tapo Camera: Port `554` (RTSP) and port `443` on `192.168.1.6`.
  - Home Assistant: Port `8123` accessible on the local network.

---

### 2. Configuration Files

Place these configuration JSON files in the working directory alongside the executable:

#### `sentinel_camera_config.json`
```json
{
  "ip": "192.168.1.6",
  "port": 0,
  "username": "admin",
  "password": "<camera-password>",
  "cloud_password": "<camera-password>",
  "brand": "tapo",
  "model": "C200",
  "stream_path": "/stream1"
}
```

#### `sentinel_ha_config.json`
```json
{
  "url": "http://192.168.1.37:8123",
  "token": "<home-assistant-long-lived-access-token>"
}
```

#### `sentinel_llm_config.json`
```json
{
  "priority_order": [
    "gemini",
    "local_qwen",
    "rule_fallback"
  ],
  "gemini_key": "<gemini-api-key>",
  "local_llm_url": "http://127.0.0.1:9931/v1/chat/completions"
}
```

#### `configs/automation.json`
```json
{
  "rules": [
    {
      "id": "turn_off_tv_idle",
      "name": "Tự động tắt TV khi vắng người",
      "icon": "tv_off",
      "enabled": true,
      "triggers": [{ "type": "idle_room", "idle_minutes": 5 }],
      "conditions": [{ "type": "tv_state", "tv_state": "on" }],
      "actions": [{ "type": "tv", "service": "turn_off", "target_tv": "192.168.1.8" }]
    }
  ]
}
```

---

### 3. Building the Binary

Compile the Go server from the repository root:

```bash
# 1. Run unit tests
go test ./...

# 2. Build for host machine
go build -o bin/sentinel_server ./agent/cmd/agent_server

# 3. Cross-compile for Linux NAS (amd64)
GOOS=linux GOARCH=amd64 go build -o /tmp/sentinel_server_linux ./agent/cmd/agent_server
```

---

### 4. Deploying to Remote NAS / Server

To deploy to a remote host (e.g. `chungnh@192.168.1.37`):

```bash
# 1. Transfer binary via SCP
scp -O /tmp/sentinel_server_linux chungnh@192.168.1.37:/home/chungnh/sentinel_server.new

# 2. Atomically update and restart process
ssh chungnh@192.168.1.37 << 'EOF'
  chmod +x /home/chungnh/sentinel_server.new
  mv /home/chungnh/sentinel_server.new /home/chungnh/sentinel_server
  pkill -f sentinel_server || true
  nohup /home/chungnh/sentinel_server > /home/chungnh/sentinel.log 2>&1 &
  sleep 1
  ps aux | grep sentinel_server
EOF
```

---

### 5. Running as a Daemon / Systemd Service

For permanent operation with automatic restarts on boot:

Create `/etc/systemd/system/sentinel.service`:
```ini
[Unit]
Description=Home Sentinel Server
After=network.target

[Service]
Type=simple
User=chungnh
WorkingDirectory=/home/chungnh
ExecStart=/home/chungnh/sentinel_server
Restart=always
RestartSec=5
StandardOutput=append:/home/chungnh/sentinel.log
StandardError=append:/home/chungnh/sentinel.log

[Install]
WantedBy=multi-user.target
```

Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now sentinel
sudo systemctl status sentinel
```

---

### 6. Verification & Health Checklist

An AI Agent must execute these verification checks after any deployment:

#### Step 1: Healthcheck
```bash
curl -s http://192.168.1.37:8788/health
# Expected: {"status":"healthy", ...}
```

#### Step 2: Conversational Chat Test (`/v2/ask`)
```bash
curl -s -X POST http://192.168.1.37:8788/v2/ask \
  -H "Content-Type: application/json" \
  -d '{"request_id":"chk-1","session_id":"s1","text":"nói với tv là Xin chào","locale":"vi-VN","input_type":"text"}'
# Expected NDJSON stream ending with "completed" status
```

#### Step 3: Check Android TV ADB Connection
```bash
adb connect 192.168.1.8:5555
adb -s 192.168.1.8:5555 shell dumpsys power | grep mWakefulness=
# Expected: mWakefulness=Awake or Asleep
```

#### Step 4: Verify Web UI Dashboard
Open `http://192.168.1.37:8788/` in a web browser. Verify:
- HA devices list loaded under "Thiết bị".
- Automation rules visible and editable under "Kịch bản".
- Camera live stream / PTZ responsive under "Camera".

---

## Communication Protocols & Endpoints

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/v2/ask` | Primary conversational AI NDJSON streaming endpoint |
| `GET` | `/v2/suggestions` | Contextual recommendations for Android TV idle state |
| `GET` | `/ws/duinch` | Real-time WebSocket connection to TV Launcher |
| `GET/POST`| `/api/automation/config` | Load, save, or test automation scenarios |
| `GET` | `/api/ha/entities` | Live Home Assistant entities list |
| `POST` | `/api/camera/ptz` | Control Tapo camera Pan/Tilt/Zoom presets |
| `POST` | `/api/camera/speak` | Send TTS audio to Tapo camera speaker |
| `GET` | `/health` | Server health check endpoint |

---

## Development Guidelines

- **File Length Limit**: Strictly $< 300$ lines per source file across all Go, JS, and CSS files.
- **Single Responsibility**: Cohesive, single-purpose modules.
- **Code Cleanliness**: No redundant comments or debug noise.
- **Validation**: Ensure all unit tests pass before deploying:
  ```bash
  go test ./...
  ```
