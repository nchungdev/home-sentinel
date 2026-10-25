# camera service

Go backend API service (`camera_center`) and Python 3 vision worker (`vision/`).

## Architecture

- **Go Camera Center (`cmd/camera_center`)**: REST server on port 8792. Handles PTZ, privacy,
  Tapo speaker TTS, go2rtc stream registration, and receives interaction events (`POST /api/events`).
  Forwards alerts and triggers directly to the TV over UDP port 8790.
- **Python Vision Worker (`vision/`)**: Reads RTMP/RTSP streams, filters room motion,
  detects person presence in zones (e.g. sofa), and dispatches webhook events to Camera Center.

| Path | Responsibility |
|---|---|
| `cmd/camera_center` | Main Go daemon binary entrypoint |
| `cmd/tapo_speaker` | CLI tool to speak directly through Tapo camera |
| `server.go` | REST HTTP API routes and request handlers |
| `sentinel.go` | Intrusion detection over go2rtc streams and TV UDP notifier |
| `tapo_ptz.go` | Tapo KLAP / ONVIF protocol for PTZ movements |
| `tts.go` | Vietnamese TTS engine and audio encoder |
| `vision/` | Ultra-low CPU Python computer vision pipeline (YOLOv8 + motion pre-filter) |

## Invariants

- Stream decoder uses low-delay buffer flags (`nobuffer`, `low_delay`).
- Motion pre-filter skips heavy YOLO inference when the room has no activity (<0.5% diff).
- Network disconnects must not crash the service; backoff reconnects automatically.
- Only class `person` triggers zone occupancy; animals and artifacts are ignored.
- An enter event requires continuous presence for `enter_debounce_seconds` (default 3s).
- A leave event requires continuous absence for `leave_debounce_seconds` (default 60s).
- Zone polygons use normalized coordinates ([0.0, 1.0]) to stay resolution-independent.
- Dispatched events contain: `zone_id`, `event_type`, `headcount`, `postures`, `timestamp`.
