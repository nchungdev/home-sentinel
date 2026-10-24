# smarthome service

High-performance Go IoT backend coordinator for home devices, rules, and TV automation.

| Module | Responsibility |
|---|---|
| `internal/gateway` | High-speed Home Assistant REST client, in-memory cache, safety guard |
| `internal/server` | Independent HTTP REST API service (`/api/devices`, `/health`) |
| `internal/model` | Canonical device model, friendly mapping for 20 switches |
| `cmd/smarthome` | Unified CLI tool and daemon entry point |

## Invariants

- Device states are cached in-memory (`sync.RWMutex`) and auto-synced every 5 seconds.
- `ct_cua_chinh_1` supplies physical power to the Hub and is locked against turn-off.
- Backend runs standalone on port 8791 and serves any external agent or UI client.
- Zero external runtime dependencies: compiled natively with standard Go libraries.
