# Server-side cache

Implemented for public internet and per-source media search: bounded memory cache by default,
optional Redis backend selected with `REDIS_URL`. Redis is not installed or started by the
application.

SearXNG cache keys include provider URL, query, language/category and schema version.
Entries expire after two minutes; only successful normalized results are stored, including genuine
empty results. Media search uses source identity, source URL, query and language in its cache key.
Entries expire after two minutes; serialized media entries larger than 128 KiB are not cached.
Missing/invalid responses, authentication failures and placeholder data are not cached.
Redis operation timeouts are bounded; cache read/write errors permit direct provider retrieval.
The memory store caps entries and per-entry size. Chat history and provider credentials are not
cached. Media results are scoped by source URL and a digest of its credential; secure Redis access
as it may contain private library metadata.
Future work: request coalescing, stale-while-revalidate, user/profile-specific private cache policy,
and real Redis service deployment/health verification. No video, cookie, token or HTML caching.
