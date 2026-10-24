# camera service

Python 3 vision worker for RTMP/RTSP streams, person presence, and zone detection.

| Module | Responsibility |
|---|---|
| `stream` | Video capture over RTMP/RTSP with low-delay flags and reconnect loop |
| `detector` | Object detection (YOLOv8 nano) filtered by class `person` |
| `zone` | Point-in-polygon test for configured zones (e.g. sofa, desk, entry) |
| `presence` | State machine with debounce timers for enter/leave events |
| `dispatcher` | Event dispatching to local agent, smarthome gateway, or MQTT |

## Invariants

- Stream decoder uses low-delay buffer flags (`nobuffer`, `low_delay`).
- Network disconnects must not crash the service; backoff reconnects automatically.
- Only class `person` triggers zone occupancy; animals and artifacts are ignored.
- An enter event requires continuous presence for `enter_debounce_seconds` (default 3s).
- A leave event requires continuous absence for `leave_debounce_seconds` (default 60s).
- Zone polygons use normalized coordinates ([0.0, 1.0]) to stay resolution-independent.
- Dispatched events contain: `zone_id`, `event_type` (`enter`|`leave`), `timestamp`.
