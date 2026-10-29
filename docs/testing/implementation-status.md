# Feature implementation status — 28/10/2026

This is the first implemented slice of the 180-case acceptance backlog. It does not claim
that the corpus passes. Original case execution_status stays not_run until a full case is verified.

## References reviewed before coding

| Repository | Stars / forks observed | Selected idea |
| --- | --- | --- |
| [SearXNG](https://github.com/searxng/searxng) | ~38.9k / 3.5k | Search-provider JSON API rather than scraping search-engine HTML |
| [HA Core](https://github.com/home-assistant/core) | ~91.8k / 39.1k | Keep device state/automation ownership outside retrieval routing |
| [LangGraph](https://github.com/langchain-ai/langgraph) | ~44k / 7.6k | Explicit task boundaries, per-task results and progressive presentation |
| [go-redis](https://github.com/redis/go-redis) | ~22.3k / 2.6k | Official Redis client with bounded operations |

These are rounded repository observations on 28/10/2026, not a quality guarantee.
Only go-redis v9.23.0 is added as a runtime dependency; the other projects are references.
SearXNG is called through its documented API; no code is copied from its AGPL implementation.
LangGraph is not installed. Existing Go application boundaries are retained.

## What now works

| Area | Actual implementation | Related acceptance cases |
| --- | --- | --- |
| Live search API | Authenticated `POST /v2/flow`; preview endpoint stays offline | TV-001/009/071/072/079 |
| Source selection | TMDb + YouTube for default title search; source-specific search when requested | TV-002/003 |
| Selected title | `context.title`; metadata/search receive normalized titles | TV-011/012/082 |
| Internet | SearXNG JSON search with valid URL filtering and source errors | TV-066/069 |
| Technical references | Topic/domain mapping and host filtering for official-doc results | TV-065 and technical routing subset |
| Film information | TMDb title search and details with director/creator fields | TV-011/012/014 |
| Media read tasks | TMDb/YouTube search; detail checks Plex/Jellyfin/*arr by exact external IDs | Search portion of TV-001/002/003/009 |
| Search cache | Per-source media and web results; memory default, optional Redis, two-minute TTL and cache error fallback | TV-093 |
| Web UI | Persistent local chat history; progressive content tabs and compact source rows | Search presentation subset |
| Stationary person | Periodic detector recheck; no motion is not an empty observation | HC-027 |
| Camera disconnect | Stale-frame rejection; observation gaps do not count toward leave debounce | Observation continuity subset of HC-025/HC-076 |

Case references above indicate the behavior addressed, not an E2E passing score.
For example, source metadata does not complete the native-TV playback path.

Default title search returns TMDb titles and YouTube videos. Selecting a TMDb title fetches
its external IDs before checking Plex, Jellyfin, Radarr, Sonarr and PhimAPI. Provider results
with a shared TMDB, IMDb or TVDB ID and matching media kind are associated as exact matches;
conflicting shared IDs reject the association. PhimAPI search results expose their own TMDB/IMDb
IDs where available. If both sides lack shared IDs, a single exact normalized title/alias with
matching year and media kind among returned search results is shown only as unverified, never as
an exact identity match.

## Configuration

The server reads these from its environment at initialization of the live search runner:

- `SEARXNG_URL`: existing SearXNG instance with JSON format enabled. No public instance is assumed.
- `TMDB_READ_ACCESS_TOKEN`: TMDb read bearer token. Never returned in responses.
- `REDIS_URL`: optional Redis connection URL; otherwise a bounded memory cache is used.
- Existing `PLEX_URL`, `JELLYFIN_URL`/API key, Radarr/Sonarr and YouTube settings remain applicable.

The web chat now tries `/v2/flow` for read/search requests and uses the old `/v2/ask` for
unsupported action/chat routes. Existing external clients keep `/v2/ask`; the launcher has
not been migrated to the new task stream. `context.title` and `context.sources` are optional;
these are search hints, not evidence of identity, permissions or source availability.

Vision adds `presence_recheck_seconds` (default 2 seconds), continues motion-triggered inference,
and rejects frames older than two seconds. There is still no full unknown/stale occupancy
contract or worker-to-HA health integration in this slice.

## Still pending

- AI semantic classifier, compound requests, robust corrections and complete context schema.
- General URL/article extraction and cited-answer synthesis.
- Remaining online adapters, season/episode flow and native launcher/player integration.
- Redis provisioning and healthy-Redis integration test; request coalescing/stale refresh.
- Camera/audio canonical events, deduplication/ordering, health propagation and permission scopes.
- Smart-home timers/scenes/automation ownership cleanup, reliable result aggregation and rule traces.
- Hardware acceptance tests for TV, real camera/model inference and device controls.

Do not mark unsupported scenarios passing just because their source/skill returns a clear error.
No live device actions, account login or automation deployment is performed by the test suite.

## Search availability correction

Live plans now omit unconfigured providers and unimplemented catalog sources by default;
explicit source requests retain a source-specific explanation. Errors are collapsed in the web UI.
PhimAPI title search is implemented and verified live with the Naruto query (8 results).
This does not implement episode playback. `PHIMAPI_URL` can override its API base URL.

## Catalog and episode slice

PhimAPI title details, source groups, special labels, stable episode IDs and fresh per-selection
resolution are now implemented behind authenticated catalog endpoints. Web search results open
an episode panel with pages of 25. Native-player descriptors use the existing streaming proxy.
This addresses parts of TV-021/022/026 and is covered by fixture and live metadata/HLS checks.
It does not complete physical-TV playback, progress/resume or next-episode acceptance.
See [catalog API](../catalog-api.md) for the current contract and remaining limitations.
