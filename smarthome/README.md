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
