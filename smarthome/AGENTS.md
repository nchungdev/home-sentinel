# smarthome service

Python 3 IoT gateway and automation coordinator for home devices and the TV.

| Module | Responsibility |
|---|---|
| `gateway` | Device abstraction for Home Assistant REST/WebSocket or local MQTT |
| `automation` | Event rules matching camera triggers to TV actions and scenes |
| `tv_bridge` | Executes TV commands (wakeup, sleep, greeting) via `tvctl` or ADB |
| `cli` | Command-line tool invoked by `services/agent` tools |

## Invariants

- Device states are cached locally to respond instantly without network stalls.
- Camera `sofa:enter` triggers TV wake and greeting only if TV is in standby.
- Camera `sofa:leave` triggers TV sleep only after the leave timeout expires.
- All TV commands route through the ADB wrapper with input sanitization.
- Failed automation dispatches are logged and never crash the gateway service.
