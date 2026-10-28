# YAMNet audio classification worker

Supporting inference process, not a separate top-level application.
`server.py` accepts length-prefixed PCM audio over TCP port 10400 and returns JSON classifications.
The Go client is `internal/speech/yamnet_client.go`.

Runtime requires Python, NumPy and ONNX Runtime. Model and class-map defaults are next to
`server.py`; override with `YAMNET_MODEL_PATH` and `YAMNET_CLASS_MAP_PATH` when mounting assets
elsewhere. Existing deployments mounting the full worker folder at `/yamnet` continue to work.

Run from the repository root with `python3 workers/yamnet/server.py` after configuring dependencies.
The worker binds to all interfaces; restrict exposure through the existing deployment network.
