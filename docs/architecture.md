# Home Sentinel Architecture & System Design

Home Sentinel is an autonomous AI agent server for smart homes and Android TV / Google TV. It serves as the intelligent backend coordinating Home Assistant, Tapo PTZ security cameras, local/cloud LLMs, Android TV ADB control, and media servers.

---

## 1. Six-Layer Pipeline Architecture

The core request pipeline follows a structured, deterministic 6-tier architecture:

```
[Input: Voice / Audio / Chat / Automation Event]
                      │
                      ▼
 1. Transport Layer (Auth, Token, Body Size Limiter, Context, NDJSON Stream)
                      │
                      ▼
 2. Understanding Layer (Vietnamese normalization, intent & slot extraction, phonetics)
                      │
                      ▼
 3. Resolution Layer (HA entity fuzzy matching, Plex search, TV ADB mapper, ActionPlan)
                      │
                      ▼
 4. Arbitration Layer (Priority admission, conflict resolution, cooldowns)
                      │
                      ▼
 5. Execution Layer (Adapters: Home Assistant, Camera Tapo PTZ & Speaker, Android TV ADB)
                      │
                      ▼
 6. Presentation Layer (Duinch Bot mood/expression, TTS speech text, client actions)
                      │
                      ▼
[Output: NDJSON Stream / WebSocket / ADB / Camera Speaker / HA Service]
```

### Layer Details

1. **Transport (`agent/pkg/transport/`)**:
   - Handles HTTP POST `/v2/ask` streaming using NDJSON (`application/x-ndjson`).
   - Body size limits, Bearer token verification, allowed IP whitelisting.
   - Real-time event streaming: `accepted` $\rightarrow$ `progress` $\rightarrow$ `presentation` $\rightarrow$ `client_action` $\rightarrow$ `completed`.

2. **Understanding (`agent/pkg/understanding/`)**:
   - Normalizes Vietnamese diacritics, common speech-to-text phonetic variations.
   - Classifies intents (`home.control`, `media.search`, `camera.ptz`, `system.status`, `general.chat`).
   - Extracts slots: target device, room, action (`turn_on`, `turn_off`, `preset`).

3. **Resolution (`agent/pkg/resolution/`)**:
   - Fuzzy matches colloquial device names against live Home Assistant entity registry.
   - Queries Plex media bridge for movie/show titles and deep links.
   - Builds deterministic `ActionPlan` containing typed `ActionStep`s.

4. **Arbitration (`agent/pkg/arbitration/`)**:
   - Resolves conflicts between simultaneous automations, physical sensors, and user commands.
   - Manages admission control and cooldown windows.

5. **Execution (`agent/pkg/adapters/`, `agent/cmd/agent_server/`)**:
   - **Home Assistant**: Direct REST calls (`/api/services/<domain>/<service>`).
   - **Camera Tapo**: RTSP 16 kHz PCM microphone stream, ONVIF PTZ preset recall, and `./bin/tapo_speaker` two-way audio.
   - **Android TV**: Network ADB commands (Wakeup `224` + `3`, Sleep `223`) and WebSocket payloads (`/ws/duinch`).
   - **Media Bridge**: Plex media search, Radarr/Sonarr download webhooks.

6. **Presentation (`orchestration_composer.go`)**:
   - Generates natural, empathetic response speech (`SpeechText`).
   - Maps responses to Duinch Bot expressions (`happy`, `excited`, `surprised`, `wink`, `sleeping`).
   - Formats `ClientActions` for remote UI widgets.

---

## 2. Automation Engine (When / And If / Then Do)

The automation system (`agent/pkg/automation/`) follows the authentic **Home Assistant 3-block model**:

```json
{
  "rules": [
    {
      "id": "idle_turn_off_tv",
      "name": "Tự động tắt TV khi vắng người",
      "icon": "tv_off",
      "enabled": true,
      "triggers": [
        { "type": "idle_room", "idle_minutes": 5 }
      ],
      "conditions": [
        { "type": "tv_state", "tv_state": "on" }
      ],
      "actions": [
        { "type": "tv", "service": "turn_off", "target_tv": "<tv-ip>" }
      ]
    }
  ]
}
```

- **When (Triggers)**:
  - `idle_room`: Room vacant for $N$ minutes (camera motion sensor).
  - `clap_sound`: Detection of distinct clapping audio from microphone.
  - `person_detected`: Person detected at specific camera PTZ preset.
  - `media_downloaded`: Radarr/Sonarr download completion event.
- **And If (Conditions)**:
  - `tv_state`: TV is currently `on` or `off` (verified via ADB `dumpsys power`).
  - `time_range`: Hour within specified window (`start_hour` to `end_hour`).
- **Then Do (Actions Pipeline)**:
  - `entities`: Turn on/off selected Home Assistant lights, switches, or fans.
  - `script`: Trigger Home Assistant scripts.
  - `tv`: ADB wake up (`turn_on`) or sleep/turn off screen (`turn_off`).
  - `camera`: Point PTZ camera to preset.

---

## 3. Communication Protocols

| Endpoint | Protocol | Description |
|---|---|---|
| `/v2/ask` | HTTP POST (NDJSON Stream) | Conversational voice/text agent processing |
| `/v2/suggestions` | HTTP GET (JSON) | Proactive context-aware suggestions for TV idle screen |
| `/ws/duinch` | WebSocket | Real-time bi-directional TV Launcher connection |
| `/api/automation/config` | HTTP GET / POST (JSON) | Automation rules CRUD & dry-run execution |
| `/api/camera/ptz` | HTTP POST (JSON) | Control Tapo camera PTZ presets and movement |
| `/api/camera/speak` | HTTP POST (JSON) | Two-way audio TTS through Tapo camera speaker |
| `/api/camera/events` | HTTP GET (JSON) | Camera motion & sound detection events log |
| `/api/ha/entities` | HTTP GET (JSON) | Cached Home Assistant entity list |
| `/api/webhook/radarr` | HTTP POST (JSON) | Radarr download webhook for TV notifications |
| `/api/webhook/sonarr` | HTTP POST (JSON) | Sonarr download webhook for TV notifications |
| `/health` | HTTP GET (JSON) | Service health check |

---

## 4. Web Dashboard (`web/`)

An ultra-lightweight, zero-dependency Single Page Application (SPA) served directly on `http://<server-ip>:8788/`:
- **Sidebar**: Quick navigation between Overview, HA Devices, Camera Monitor, Automation Rules, and System Logs.
- **Automation Rule Builder**: HA-style graphical editor with collapsible cards, step badges, multi-select entity chips, live search filter, and individual rule test triggers.
- **Live Terminal**: Streaming log output from camera background listener, wake word detector, and ADB dispatchers.
