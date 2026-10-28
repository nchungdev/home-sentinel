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
  - [4. Deploying to Remote Server / NAS](#4-deploying-to-remote-server--nas)
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

```text
home-sentinel/
├── cmd/
│   ├── sentinel/               # Main Home Sentinel server
│   ├── camera-center/          # Optional camera connector daemon
│   └── tapo-speaker/           # Camera speaker utility
├── internal/
│   ├── contracts/             # Typed request/action/result contracts
│   ├── requestflow/           # Classification, planning, skill execution preview
│   ├── orchestrator/          # Existing request pipeline
│   ├── understanding/         # Existing intent and slot extraction
│   ├── resolution/            # Entity matching and action planning
│   ├── tools/                 # Explicit action tools and registry
│   ├── skills/                # Read/search skills and metadata retrieval
│   ├── search/web/            # General internet search through SearXNG
│   ├── mediasearch/           # Existing multi-source media search adapters
│   ├── catalog/               # PhimAPI title details and episode resolution
│   ├── playback/              # Stream resolution and playback sessions
│   ├── watching/              # Watch history and progress
│   ├── cache/                 # Bounded memory and optional Redis search cache
│   ├── integrations/          # Camera, HA, media-server and ONVIF connectors
│   ├── adapters/              # Execution adapters for device actions
│   ├── llm/                   # AI engines, credentials and login flows
│   ├── speech/                # Audio service clients and transcription
│   ├── transport/             # HTTP/NDJSON protocol handlers
│   └── ...                    # Arbitration, automation, proactive, telemetry
├── workers/
│   ├── vision/                # Camera image processing worker
│   └── yamnet/                # Audio classification worker and model
├── web/                       # Embedded web dashboard
├── configs/                   # Configuration examples and automation rules
├── scripts/                   # Operational command-line helpers
├── deploy/                    # Deployment files, systemd and supporting services
├── docs/                      # Architecture, setup and acceptance cases
└── tests/                     # Cross-component test support
```

Home Sentinel is the application at the repository root. Camera connectors and inference
workers supply data or processing capabilities to it; they are optional supporting components.
Folders marked planned contain responsibility specifications, not working implementations.
See [repository layout](docs/repository-layout.md) for boundaries and migration paths.

---

## Home Sentinel Installation & Setup Guide

> **Note for AI Agents**: Follow these sequential instructions when setting up, compiling, deploying, or troubleshooting the Home Sentinel server.

### 1. Prerequisites

Verify that the target machine (NAS / Linux server) has the following dependencies:
- **Architecture**: Linux `x86_64` (`amd64`) or `aarch64` (`arm64`).
- **FFmpeg**: Required for camera audio stream processing (`ffmpeg -version`).
- **ADB**: Required for Android TV control (`adb version` or path to binary).
- **Network Access**:
  - Android TV: Port `5555` open on `<tv-ip>`.
  - Tapo Camera: Port `554` (RTSP) and port `443` on `<camera-ip>`.
  - Home Assistant: Port `8123` accessible on `<ha-ip>`.

---

### 2. Configuration Files

Place these configuration JSON files in the working directory alongside the executable:

#### `sentinel_camera_config.json`
```json
{
  "ip": "<camera-ip>",
  "port": 0,
  "username": "<camera-username>",
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
  "url": "http://<ha-ip>:8123",
  "token": "<home-assistant-long-lived-access-token>"
}
```

#### `sentinel_llm_config.json`
```json
{
  "selected_engine": "gemini",
  "gemini_key": "<gemini-api-key>",
  "local_llm_url": "http://127.0.0.1:9931/v1/chat/completions"
}
```

Sentinel runs on the host you choose (Linux/NAS, macOS or Windows).
Set `AGENT_BIND=127.0.0.1` for a local-only instance; the default listens on all interfaces.
Select one engine in Settings; requests use only that engine and report failures without
switching providers. Existing `priority_order` files migrate to their first recognized engine.
`rule_fallback` now means AI chat is disabled.

Gemini, OpenAI API and Anthropic API accept keys in Settings. OpenAI and Anthropic also
require a model ID available to the API account. Blank key fields retain saved credentials;
GET responses expose only whether a key is set. Configuration files are saved with owner-only
permissions where supported by the operating system.

CLI engines reuse authentication on the host running Sentinel under the service's OS account.
Settings checks that existing session before starting any login and exposes each engine's methods:
Codex (ChatGPT, API key, organization access token), Claude (subscription, Console, SSO, API key,
OAuth token, existing cloud configuration), and AGY (Google/Gemini Enterprise, Gemini key, ADC).
Google Cloud ADC login requires `gcloud` on the server. API-only engines retain their key form.

New browser logins execute on the server and expose the CLI-generated URL to the web client.
Codex uses device authentication. AGY receives a terminal (PTY on Unix, ConPTY on Windows),
with terminal queries answered and a separate empty workspace for its startup trust prompt.
Claude browser launches are captured by a Sentinel helper rather than opening a browser on the
server. A client on another device can paste a loopback callback URL into the login form;
Sentinel forwards only the callback matching that session's loopback target and OAuth state.

Login completion is verified against the native session, rather than exit code alone. Logins can
be cancelled and expire after five minutes. UI reloads resume an in-progress session. Keys and
tokens are stored only in native CLI storage or the owner-only local Sentinel configuration.
Connection tests send a small model request without saving drafts.

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
      "actions": [{ "type": "tv", "service": "turn_off", "target_tv": "<tv-ip>" }]
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
go build -o bin/sentinel_server ./cmd/sentinel

# 3. Cross-compile for Linux target (amd64)
GOOS=linux GOARCH=amd64 go build -o /tmp/sentinel_server_linux ./cmd/sentinel
```

---

### 4. Deploying to Remote Server / NAS

To deploy to a remote host (e.g. `<user>@<server-ip>`):

```bash
# 1. Transfer binary via SCP
scp -O /tmp/sentinel_server_linux <user>@<server-ip>:/home/<user>/sentinel_server.new

# 2. Atomically update and restart process
ssh <user>@<server-ip> << 'EOF'
  chmod +x /home/<user>/sentinel_server.new
  mv /home/<user>/sentinel_server.new /home/<user>/sentinel_server
  pkill -f sentinel_server || true
  nohup /home/<user>/sentinel_server > /home/<user>/sentinel.log 2>&1 &
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
User=<user>
WorkingDirectory=/home/<user>
ExecStart=/home/<user>/sentinel_server
Restart=always
RestartSec=5
StandardOutput=append:/home/<user>/sentinel.log
StandardError=append:/home/<user>/sentinel.log

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
curl -s http://<server-ip>:8788/health
# Expected: {"status":"healthy", ...}
```

#### Step 2: Conversational Chat Test (`/v2/ask`)
```bash
curl -s -X POST http://<server-ip>:8788/v2/ask \
  -H "Content-Type: application/json" \
  -d '{"request_id":"chk-1","session_id":"s1","text":"nói với tv là Xin chào","locale":"vi-VN","input_type":"text"}'
# Expected NDJSON stream ending with "completed" status
```

#### Step 3: Check Android TV ADB Connection
```bash
adb connect <tv-ip>:5555
adb -s <tv-ip>:5555 shell dumpsys power | grep mWakefulness=
# Expected: mWakefulness=Awake or Asleep
```

#### Step 4: Verify Web UI Dashboard
Open `http://<server-ip>:8788/` in a web browser. Verify:
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
| `GET` | `/api/nas/status` | Disk, Docker and host snapshot of the NAS (JSON, no LLM) |
| `POST` | `/api/token/generate` | Rotate the access token (409 when `AGENT_TOKEN` manages it) |
| `POST` | `/api/auth/login` | Web login: `{"token": "..."}` sets an HttpOnly session cookie |
| `GET` | `/api/auth/status` | `{"required": bool, "authorized": bool}` |
| `GET` | `/health` | Health check; 401 when a wrong `Authorization` header is sent |

### Access control

Every route except `/`, `/health` and `/api/auth/*` requires the access token, sent as
`Authorization: Bearer <token>` (TV launcher), the `sentinel_session` cookie (web UI) or the
HTTP Basic password (Radarr/Sonarr webhooks). The token comes from `AGENT_TOKEN`, otherwise
from `sentinel_auth.json` written by `/api/token/generate`; without either, the server runs
open and logs a warning. `AGENT_ALLOW` limits every route to the listed IPs or CIDRs.
`AGENT_NAS_MOUNTS` (comma-separated, default `/`) and `AGENT_DOCKER_SOCKET` choose what
`/api/nas/status` and the "trạng thái NAS" intent report.

---

## Development Guidelines

- **File Length Limit**: Strictly $< 300$ lines per source file across all Go, JS, and CSS files.
- **Single Responsibility**: Cohesive, single-purpose modules.
- **Code Cleanliness**: No redundant comments or debug noise.
- **Validation**: Ensure all unit tests pass before deploying:
  ```bash
  go test ./...
  ```
