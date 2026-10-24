# AI Camera & Presence Detection Service

Monitors an RTMP or RTSP video stream, tracks user presence in defined zones
(such as the living room sofa), and triggers events for the TV and smart home.

## Features

- **Zone presence**: Detects when someone sits on the sofa or leaves the room.
- **Debounced triggers**: Prevents flapping (e.g. requires sitting 3s, absent 60s).
- **Direct integration**: Dispatches events to `services/agent` or `services/smarthome`.

## Setup

```bash
cd services/camera
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
cp camera_config.example.json camera_config.json
```

Edit `camera_config.json` with your stream URL and sofa coordinates:

```json
{
  "stream_url": "rtmp://192.168.1.100/live/sofa",
  "model": "yolov8n.pt",
  "zones": [
    {
      "id": "sofa",
      "polygon": [[0.3, 0.4], [0.7, 0.4], [0.7, 0.8], [0.3, 0.8]],
      "enter_debounce_seconds": 3,
      "leave_debounce_seconds": 60
    }
  ],
  "webhook_url": "http://127.0.0.1:8788/event"
}
```

## Running

```bash
python3 main.py
```
