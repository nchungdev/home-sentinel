# Services

Backend services, AI servers, and background workers for the TV ecosystem:

| Service | Description | Tech Stack | Status | Docs |
|---|---|---|---|---|
| `agent/` | Voice assistant & media hub | Python 3 (HTTP/SSE) | Active | `README.md` |
| `camera/` | AI camera & zone presence | Python / YOLO / RTMP | In Dev | `AGENTS.md` |
| `smarthome/` | Device coordinator & rules | Python / Home Assistant | In Dev | `AGENTS.md` |

## Architecture

- **Independent deployment:** Each service runs on a home server (NAS, Mini PC, Pi) or host.
- **Event-driven presence:** `camera/` publishes presence events (e.g. sofa enter/leave) to
  trigger automation rules in `smarthome/`.
- **Agent orchestration:** `agent/` uses CLI tools in `services/*/` to query status and control
  appliances upon voice commands.
- **Lightweight communication:** TV connects via HTTP/SSE or WebSocket with token-based
  authentication.
