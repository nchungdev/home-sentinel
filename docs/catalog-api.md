# PhimAPI catalog and episode playback

## References

Reviewed before implementation:
[Stremio Addon SDK](https://github.com/Stremio/stremio-addon-sdk) (~1.4k stars / 449 forks),
[Jellyfin](https://github.com/jellyfin/jellyfin) (~58.7k stars / 5.7k forks), and the
[first-party PhimAPI documentation](https://www.kkphim2.com/api-document).
The useful boundary is title metadata → episode identity → stream resolution; no SDK code is copied.

## Endpoints

Both endpoints use the server's existing authentication and network policy.

| Method and path | Response |
| --- | --- |
| `GET /v2/catalog/titles/{id}` | `{ "title": Title }` |
| `POST /v2/catalog/episodes/{id}/playback` | `{ "episode": Episode, "playback": Descriptor }` |

The POST body is empty or `{}`. Arbitrary URLs are not accepted as parameters.
URL-encode path IDs. A known title example is `phimapi:naruto-shippuden`.
Get episode IDs from the title response instead of constructing them on the client.

Search remains `POST /v2/flow`. PhimAPI results now include a `catalog_detail` action with
`item_id` identifying the title. Older clients can still use the external page URL.

## Data

Title: id, source_id, name, kind, optional year, seasons and episodes arrays.
Season: id, label and order, only when the provider actually supplies season data.
Episode: id, title_id, label, group, order, optional season_id and playable.

The ID includes the source/title/playback-group/episode identity and excludes the stream URL.
An encoded ID is not a secret or authorization grant. The source must still contain the episode
when resolving. Specials retain their original labels; ordering follows provider data.
PhimAPI Naruto Shippuden currently supplies 500 episodes in one Vietsub group and no season data.

Metadata responses contain no upstream stream links. Only normalized metadata is cached for
15 minutes, subject to bounded cache size; errors are not cached. Cache keys include source API
identity and schema version. Resolving always reads current provider data, independently of cache.

Descriptor contains the scoped session URL, MIME type, title, preferred_handler, preferred_players
and internal session expires_at. The URL is relative to the server; native clients resolve it
against their configured server origin. Internal expiry does not guarantee upstream URL lifetime.
The current implementation proxies media; direct-client routing is future work.

## Error behavior

- 400 invalid_catalog_id / invalid body: malformed or unsafe input.
- 404 catalog_item_not_found: provider no longer contains the title/episode.
- 422 episode_playback_unsupported: no supported direct stream or unsafe stream host.
- 503 catalog_source_unavailable: source/session service cannot serve the request.
- 504 catalog_timeout: request deadline exceeded.
- 502 catalog_source_failed: invalid or otherwise failed provider response.

The playback resolver rejects private, shared-address and reserved network targets.
Redirects and HLS child resources use the existing DNS-pinned network client.
Only HLS/MP4 direct media is supported; an embed page is not treated as a stream.
The proxy supports HLS GET/HEAD, rewrites resource links and forwards byte ranges.
Media bytes are streamed rather than buffered as a whole video.

## Web interaction

PhimAPI search-result titles open an inline episode panel, retaining the search results.
Choose the provider group and use pages of 25 episodes; 500 episodes do not create 500 buttons.
Closing restores focus to the search-result button. A newer playback selection cancels the prior
request and cannot be overwritten by its late response.

Selecting an episode requests a fresh descriptor. The web panel offers a copyable player URL.
Android browsers also get an explicit VLC intent link; this does not detect whether VLC is installed.
No source website or WebView is loaded to obtain the video.

## Verification and remaining work

Verified live on 28/10/2026 through the running server: 8 Naruto search results with detail actions,
500 Naruto Shippuden episodes, stable first-episode ID, playback descriptor and HTTP 200 HLS manifest.
Fixtures cover metadata cache/stream refresh separation, special labels, server-group identity,
forged IDs, embed-only streams, private-network rejection, HLS rewriting, HEAD and ranges.

Native launcher episode UI, installed-player fallback, per-episode progress, resume, next episode,
upstream-expiry recovery, session lifecycle cleanup and second-provider validation remain pending.
The internal 6-hour/32-session playback limit still applies; expired sessions are collected on create.
This is not a claim that a physical TV has completed playback or that all 180 acceptance cases pass.
