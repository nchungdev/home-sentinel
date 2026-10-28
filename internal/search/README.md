# Search

General internet search is implemented in `web/`, with SearXNG as the initial provider.
Existing library/catalog/video search adapters remain in `internal/mediasearch/`.

Select providers after request understanding/planning, not by treating every input as media.
Preserve source, URL and independent provider errors. Deliver fast results before slow providers.
Catalog and library adapters retain domain-specific records outside this folder.
