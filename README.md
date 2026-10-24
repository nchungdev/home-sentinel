# Services

Backend services, AI servers, and background workers for the TV ecosystem:

| Service      | Description                           | Tech Stack          | Status | Docs        |
|--------------|---------------------------------------|---------------------|--------|-------------|
| `agent/`     | Voice assistant & media hub           | Python 3 (HTTP/SSE) | Active | `README.md` |
| `camera/`    | AI camera & zone presence             | Go / Python / Tapo  | Active | `AGENTS.md` |
| `smarthome/` | Device coordinator & rules            | Go / Python / HA    | Active | `AGENTS.md` |
| `hub/`       | Legacy hub, replaced by `gateway/`    | Go / Web / Embedded | Legacy | |
| `gateway/`   | TV pairing & generic device API over Home Assistant | Go | Active | `README.md` |

## Architecture

- **Independent deployment:** Each service runs on a home server (NAS, Mini PC, Pi) or host.
- **Event-driven presence:** `camera/` publishes presence events (e.g. sofa enter/leave) to
  trigger automation rules in `smarthome/`.
- **Agent orchestration:** `agent/` uses CLI tools in `services/*/` to query status and control
  appliances upon voice commands.
- **Lightweight communication:** TV connects via HTTP/SSE or WebSocket with token-based
  authentication.
