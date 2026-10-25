# services

| Service | Responsibility | Read |
|---|---|---|
| `agent/` | AI sessions, media and TV tools | `README.md` |
| `camera/` | Stream handling, vision and camera-specific controls | `AGENTS.md` |

- Services run independently on the local network.
- Launcher connects directly to Home Assistant; HA owns device state and automation.
- DQHome authentication belongs to HA's Remote Home-Assistant integration.
- Camera presence events may feed HA automations; do not assume a receiver is configured.
- Agent must use explicit tools for device actions and report their actual results.
- Do not restore dependencies on the retired Hub, Gateway or Smart Home services.
- Credentials remain in local environment/configuration files, never bundled in artifacts.
