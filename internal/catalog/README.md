# Catalog

Implemented for PhimAPI: normalized title details, ordered episode groups, stable episode IDs
and fresh direct-stream resolution on selection. API composition is in `cmd/sentinel/handlers_catalog.go`.

- Title IDs use source namespace and slug.
- Episode IDs encode source title slug, playback group and episode slug; stream URLs are excluded.
- Preserve provider labels, including specials. A playback group is not a season.
- Sources without season metadata return an empty seasons array.
- Only normalized title/episode metadata is cached, using memory or configured Redis.
- ResolveEpisode always refetches the provider; metadata TTL does not imply stream validity.
- Embed-only resources remain listed but are not advertised as directly playable.
- Invalid source schemas, duplicate episode identities and unsafe IDs return explicit errors.

Search still lives in `internal/mediasearch/phimapi.go`. Playback sessions, public-host validation
and HLS resource proxying belong to `internal/playback/`. Watch history remains separate.

Other providers, automatic URL expiry recovery and native launcher integration are not implemented here.
