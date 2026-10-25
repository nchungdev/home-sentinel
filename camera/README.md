# Camera Service (Command Center & Vision)

Two components optimized for minimal CPU & RAM footprint:
1. **Camera Command Center (Go)**: Web API on port 8792 for PTZ, privacy mode, Tapo TTS speaker,
   and receiving camera interaction events (`POST /api/events`) to broadcast UDP to Android TV.
2. **Vision Worker (Python)**: Low-overhead YOLOv8 / motion pre-filtering pipeline in `vision/`.

## Running Camera Command Center (Go)

```bash
cd camera && go run ./cmd/camera_center
```

## Running Vision Worker (Python)

```bash
cd services/camera/vision
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
cp camera_config.example.json camera_config.json
python3 main.py
```

`camera_config.json` points to the Go Command Center event webhook:

```json
{
  "stream_url": "rtsp://192.168.1.6:554/stream1",
  "model": "yolov8n.pt",
  "zones": [
    {
      "id": "sofa",
      "polygon": [[0.3, 0.4], [0.7, 0.4], [0.7, 0.8], [0.3, 0.8]],
      "enter_debounce_seconds": 3,
      "leave_debounce_seconds": 60
    }
  ],
  "webhook_url": "http://127.0.0.1:8792/api/events"
}
```
