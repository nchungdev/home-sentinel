# services

Backend and ambient services for the TV ecosystem: AI assistant, vision, and smart home.

| Service | Responsibility | Read |
|---|---|---|
| `services/agent/` | Voice assistant (Duinch), LLM sessions, tools caller (`tv`) | `README.md` |
| `services/camera/` | RTMP/RTSP stream ingest, YOLO detection, zone presence | `AGENTS.md` |
| `services/smarthome/` | Home Assistant / MQTT bridge, automation, TV sleep/wake | `AGENTS.md` |

## Invariants

- Services run as independent processes (or user systemd units) on the local network.
- `camera` only detects and emits presence events; it does not directly control devices.
- `smarthome` executes automations based on presence events and exposes CLI tools.
- `agent` interacts with devices strictly through tool calls (e.g. `tv`, `smarthome`).
- Communication between local services uses HTTP webhooks or MQTT.
- Sensitive credentials (tokens, stream URLs with passwords) live only in `.env` or `config.json`.
