# Smart Home Gateway & Automation Service

Coordinates smart home devices, listens to camera presence triggers,
and provides control tools for the AI agent on the TV.

## Architecture

```
[ Camera Service ] ──── (Webhook / MQTT) ───┐
                                            ▼
[ AI Agent Card / Voice ] ── (CLI Tool) ──> [ Smarthome Service ]
                                            │
                                            ├──> Home Assistant / MQTT
                                            └──> TV Control (tvctl / ADB)
```

## Configuration

Copy and edit `smarthome_config.example.json`:

```json
{
  "tv_host": "192.168.1.50:5555",
  "port": 8791,
  "home_assistant": {
    "url": "http://homeassistant.local:8123",
    "token": "YOUR_HA_LONG_LIVED_TOKEN"
  },
  "rules": {
    "on_sofa_enter": {
      "tv_action": "wake",
      "greeting": "Xin chào! Chúc bạn xem phim vui vẻ."
    },
    "on_sofa_leave": {
      "tv_action": "sleep"
    }
  }
}
```

## Agent Tool Integration

The agent server in `services/agent` invokes `services/smarthome/bin/smarthome`
as a CLI tool to query status or toggle devices.

## Documentation

For full REST API reference, request/response schemas, and device catalogue, see:
[`docs/smarthome-api.md`](../../docs/smarthome-api.md)

## Go Backend Service

The standalone Go backend server (`bin/smarthome`) manages devices directly via Home Assistant:

```bash
make smarthome                                       # Compile binary to bin/smarthome
services/smarthome/bin/smarthome serve --port 8791   # Run backend API server
services/smarthome/bin/smarthome status              # Query all 20 devices in JSON
services/smarthome/bin/smarthome toggle ct_pn_nho_1  # Toggle specific switch
services/smarthome/bin/smarthome set ct_pk_1_1 on    # Set state (on/off)
```

### REST API Endpoints

- `GET /health` - Service health, gateway connection, total device count
- `GET /api/devices` - Current list of 20 devices and real-time states
- `GET /api/devices/{id}` - Single device details
- `POST /api/devices/{id}/state` - Set state (`{"state": "on"|"off"}`)
- `POST /api/devices/{id}/toggle` - Toggle current state
- `POST /api/sync` - Force immediate synchronization with gateway
