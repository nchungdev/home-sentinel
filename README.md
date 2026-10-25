# Services

Only two optional TV Launcher backends remain:

| Service | Responsibility | Default port |
|---|---|---|
| `agent/` | AI conversations, media queries and TV tools | 8788 |
| `camera/` | Camera streams, vision, PTZ, privacy and speaker tools | 8792 |

Launcher Smart Home connects directly to Home Assistant. Home Assistant connects to
DQHome through Remote Home-Assistant using a DQHome token. Neither connection needs
a TV Launcher Hub, Gateway or Smart Home backend.

The former `gateway/`, `hub/` and `smarthome/` services were removed on 2026-10-25.
Agent no longer forwards smart home requests to Hub. A direct Home Assistant tool
for Agent must be configured separately; the launcher's HA token stays on the TV.

See [direct HA setup](../docs/home-assistant-direct.md) and each service's README.
