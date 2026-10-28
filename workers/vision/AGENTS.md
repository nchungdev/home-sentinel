# Vision processing worker

This worker supplies camera observations to Home Sentinel or configured webhook consumers.
Camera device control code is in `internal/integrations/camera/`; setup is in
`docs/integrations/camera.md`.

- Keep low-delay decoding and motion pre-filtering; periodically recheck stationary presence.
- Skipped inference and stale/missing frames are not empty-person observations.
- Observation gaps must not count toward enter/leave debounce intervals.
- Handle disconnects with bounded backoff; do not crash on a transient stream failure.
- Only person detections update occupancy; preserve enter/leave debounce behavior.
- Zone polygons use normalized coordinates.
- Preserve event fields: zone_id, event_type, headcount, postures and timestamp.
- Do not assume a webhook receiver or Home Assistant automation is already configured.
