# Acceptance scenario corpus

These documents define proposed behavior; scenario execution is tracked separately.
All JSON cases currently have `execution_status: not_run`.

| Corpus | Cases | Document | Structured fixture |
| --- | --- | --- | --- |
| TV user requests | 100 | [TV cases](tv-user-cases.md) | [JSON](tv-user-cases.json) |
| Smarthome, camera, presence and audio | 80 | [Home/camera cases](smarthome-camera-cases.md) | [JSON](smarthome-camera-cases.json) |

The combined corpus contains 180 scenarios with separate ID namespaces TV-* and HC-*.
Some concepts intentionally overlap: TV cases test user-facing requests; HC cases deepen event,
condition, authorization, concurrency, source-health and recovery behavior.
These are not 180 statistically independent use cases or 180 passing automated tests.

Implementation progress and verified boundaries: [feature status](implementation-status.md).
