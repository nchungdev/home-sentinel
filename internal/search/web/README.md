# General internet search

Implemented: SearXNG JSON search using server-side `SEARXNG_URL`; title/URL/snippet results
are independent of the media domain. Optional Redis or bounded memory caching is in `internal/cache/`.
Each provider request is bounded by timeout and response size; redirects are not followed.
Invalid/credential-bearing/non-HTTP URLs and duplicate result URLs are dropped.

`internal/skills/documentation.go` maps recognized technical topics to official domains and
filters returned hosts before labeling a result official. It is not a universal authority detector.

Not implemented: arbitrary page extraction, article summarization, browser-backed extraction,
search-to-extraction dependency scheduling and answer synthesis. Links do not imply playable video.
The existing media-only SearXNG source remains for the separate `/api/media/search` API.
