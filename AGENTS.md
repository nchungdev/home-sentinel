# services

| Service | Responsibility | Read |
|---|---|---|
| `agent/` | AI sessions, media and TV tools | `README.md` |
| `camera/` | Stream handling, vision and camera-specific controls | `AGENTS.md` |

- Services run independently on the local network.
- Launcher connects directly to Home Assistant; HA owns device state and automation.
- DQHome authentication belongs to HA's Remote Home-Assistant integration.
- Camera presence events may feed HA automations; do not assume a receiver is configured.
- Agent must use explicit tools for device actions and report their actual results.
- Do not restore dependencies on the retired Hub, Gateway or Smart Home services.
- Credentials remain in local environment/configuration files, never bundled in artifacts.

## Code convention

- Clean code without comments (generated-file headers excepted); self-explanatory names,
  guard clauses, early returns and small single-purpose functions carry the meaning.
- Names say what a thing is: `serviceStream`, `negativeInverse`, `remoteWallpaper`, never
  `s`, `n`, `tmp`. One-letter names only for Go method receivers.
- Related statements form a block; a blank line separates blocks that do different things
  (after a finished `if`/`for`/`switch`, before the final `return`).
- Every source file stays under 300 lines (generated files excepted). Split growing files by
  cohesive responsibility before adding new features.
- Lines stay within 100 characters (tabs count as 4). Long messages become named constants
  built from short string parts; long argument lists continue on the next line.
- SOLID and DRY:
  - Single Responsibility: one reason to change per function, class and file.
  - Open/Closed: extend behaviour via composition, strategies or config rather than branching.
  - Liskov Substitution: implementations honour their interface contracts without surprises.
  - Interface Segregation: small, focused interfaces defined by the caller.
  - Dependency Inversion: core logic depends on abstractions for fast, deterministic unit tests.
  - DRY only where it removes real duplication; prefer clear local code over premature
    abstraction.
