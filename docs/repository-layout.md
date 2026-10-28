# Home Sentinel repository layout

Home Sentinel is the application rooted in this repository. AI request handling is its core.
Camera integrations and vision/audio classifiers provide input data or processing capabilities;
they do not define sibling products. Supporting processes can still run on separate hosts.

## Boundaries

| Folder | Responsibility | Current state |
| --- | --- | --- |
| `cmd/sentinel/` | Main HTTP server composition and entrypoint | Existing server moved |
| `cmd/camera-center/` | Optional camera connector daemon | Existing daemon moved |
| `cmd/tapo-speaker/` | Camera speaker command | Existing utility moved |
| `internal/contracts/` | Typed request, action and response contracts | Implemented |
| `internal/understanding/` | Existing intent/slot classification | Rule-based implementation |
| `internal/orchestrator/` | Existing request pipeline | Implemented |
| `internal/requestflow/` | Classification, planning and task streaming | Preview and live read/search skills |
| `internal/skills/` | Read/search skills and metadata | Implemented; registry remains in requestflow |
| `internal/tools/` | Explicit action tools | Existing registry |
| `internal/resolution/` | Entity resolution and action plans | Implemented |
| `internal/mediasearch/` | Library, catalog and video search | Partial adapters and source placeholders |
| `internal/search/web/` | General internet discovery | SearXNG implemented; page extraction pending |
| `internal/catalog/` | Film details and stable episode identities | PhimAPI detail/resolve implemented |
| `internal/playback/` | Stream extraction, proxy sessions and native player descriptors | Partial implementation |
| `internal/watching/` | Watch progress/history and continue-watching data | Existing implementation |
| `internal/cache/` | Search caching | Memory and Redis clients implemented; Redis not provisioned |
| `internal/integrations/` | Camera, streams, HA, media servers, ONVIF | Existing connectors moved |
| `internal/adapters/` | Device-action execution adapters | Existing implementation |
| `internal/llm/` | AI engines, native account sessions and API credentials | Existing implementation |
| `internal/speech/` | Transcription and audio inference clients | Existing implementation |
| `internal/transport/` | HTTP/NDJSON transport | Existing implementation |
| `workers/vision/` | Person/motion/zone observations | Existing Python worker moved |
| `workers/yamnet/` | Audio classifications and ONNX assets | Existing Python worker moved |
| `web/` | Embedded web interface | Existing location |
| `configs/` | Configuration examples and checked-in rules | Existing plus moved example |
| `deploy/` | Systemd and supporting-service deployment files | Existing plus moved unit |
| `scripts/` | Operational CLI helpers | Existing TV helper moved |
| `docs/testing/` | 100 TV + 80 smarthome/camera scenarios | Specifications, not executed tests |

## Dependency direction

Entrypoints compose the application. Request planning uses contracts and registered skills/tools.
Device and external-service code lives behind integration/adapter boundaries. Workers send
observations or bounded processing results through their protocols; they do not own request routing.
The web UI consumes server responses. An integration must not import an executable entrypoint.

All application Go packages are `internal`, so external modules cannot depend on implementation
packages. The module identity remains `github.com/nchungdev/home-sentinel`.

This migration changes placement and import paths. It does not split existing packages into
new abstractions merely to fill folders. Planned folders contain explicit responsibility specs;
implementation moves there when a real feature requires it.

## Path migration

| Old path | New path |
| --- | --- |
| `agent/cmd/agent_server/` | `cmd/sentinel/` |
| `agent/pkg/` | `internal/` |
| `agent/pkg/camstream/` | `internal/integrations/camstream/` |
| `agent/pkg/haclient/` | `internal/integrations/haclient/` |
| `agent/pkg/mediabridge/` | `internal/integrations/mediabridge/` |
| `agent/pkg/onvifscan/` | `internal/integrations/onvifscan/` |
| `camera/pkg/camera/` | `internal/integrations/camera/` |
| `camera/cmd/camera_center/` | `cmd/camera-center/` |
| `camera/cmd/tapo_speaker/` | `cmd/tapo-speaker/` |
| `camera/vision/` | `workers/vision/` |
| `yamnet_service/` | `workers/yamnet/` |
| `agent/agent_config.example.json` | `configs/agent.example.json` |
| `agent/tv-agent.service` | `deploy/systemd/tv-agent.service` |
| `agent/tv` | `scripts/tv` |
| `agent/README.md` | `docs/server-setup.md` |
| `camera/README.md` | `docs/integrations/camera.md` |

Old source paths above are retained only as migration documentation, not runnable instructions.
Binary names and network protocols remain unchanged. Runtime credentials, config files and the
ignored `local/` directory are not migrated. External automation referring to old source paths
must switch to the paths above; there are no compatibility symlinks.

## Build and verification

Run from the repository root:

```sh
go build -o bin/sentinel_server ./cmd/sentinel
go build -o bin/camera_center ./cmd/camera-center
go build -o bin/tapo_speaker ./cmd/tapo-speaker
go test ./...
python3 -m unittest discover -s workers/vision -p 'test_*.py'
```

Vision setup: `docs/integrations/camera.md`. Server setup: `docs/server-setup.md`.
Worker inference needs its own model/runtime dependencies; Go tests do not verify Python model
inference or hardware stream access. A file move does not deploy or restart running services.
