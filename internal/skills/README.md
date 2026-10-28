# Retrieval skills

Implemented skills: per-source library/catalog/YouTube search via existing media adapters,
general internet search through SearXNG, official-domain technical search, and TMDb title metadata
with director/creator information. Current task routing and Registry remain in `internal/requestflow/`.

Registered skills are wired in `cmd/sentinel/search_flow.go`. Missing credentials/configuration
produce explicit source statuses; catalog website placeholders remain unimplemented.
Explicit device-action tools still live in `internal/tools/` and the existing `/v2/ask` path.
The new search endpoint does not execute writes or deploy automations.

Not implemented: article extraction, model-driven skill selection/answer synthesis and the remaining
specialized skills in the acceptance corpus. Do not import local Codex skills into the server.
