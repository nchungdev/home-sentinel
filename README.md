# Services

Backend services, AI servers, and background workers for the TV ecosystem:

| Service | Description | Tech Stack | Status |
|---|---|---|---|
| `agent/` | Voice assistant (Duinch) & media hub | Python 3 (HTTP/SSE) | Active |
| `camera/` | AI camera monitoring & RTSP streaming | Python / RTSP / Vision | Planned |
| `smarthome/` | Home automation & device status worker | Python / Home Assistant | Planned |

## Architecture

- **Independent deployment:** Each service is designed to run on a home server (NAS, Mini PC, Raspberry Pi) or cloud host.
- **Lightweight communication:** TV connects via HTTP/SSE or WebSocket with token-based authentication.
- **Modular configuration:** Each service manages its own environment variables and optional configuration files.
