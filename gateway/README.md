# gateway

Go service between the TV launcher and Home Assistant. Home Assistant owns accounts,
devices and areas; the gateway only pairs TVs and turns Home Assistant entities into a
generic device API.

```
HA_URL=http://nas.local:8123 PORT=8795 DATA_DIR=/data ./gateway
```

| Variable | Meaning |
|---|---|
| `HA_URL` | Home Assistant base URL reachable from the gateway (required) |
| `HA_PUBLIC_URL` | Same server as the browser sees it, when it differs from `HA_URL` |
| `GATEWAY_URL` | Gateway base URL as the browser sees it, when behind a proxy |
| `PORT` | Listen port, default 8795 |
| `DATA_DIR` | Where `sessions.json` lives (holds Home Assistant refresh tokens, mode 0600) |

## Pairing (quick login)

1. TV: `POST /api/v1/pair/request {"name"}` returns `request_id`, `code`, `approve_url`.
2. TV shows the code and polls `GET /api/v1/pair/status?request_id=`.
3. A person opens `/pair` on the gateway, checks the code, presses Approve and signs in to
   Home Assistant (its own login page, OAuth authorization code flow).
4. The gateway keeps that user's Home Assistant refresh token and hands the TV an opaque
   gateway token, delivered once by the status poll.

The TV has the permissions of the user who approved it, and every call to Home Assistant is
made with that user's token. `DELETE /api/v1/session` or removing the token in the Home
Assistant profile revokes the TV.

## Device API

All calls send `Authorization: Bearer <gateway token>`.

| Call | Purpose |
|---|---|
| `GET /api/v1/devices` | `{version, areas[], devices[]}` |
| `GET /api/v1/devices/{id}` | one device |
| `POST /api/v1/devices/{id}/actions` | `{"capability", "value"}` returns the updated device |
| `GET /api/v1/devices/{id}/stream` | camera stream, proxied with the user's token |
| `DELETE /api/v1/session` | log this TV out |

A device is `{id, name, type, icon, area, available, capabilities[], state{}}`. `state` is
keyed by capability id. A capability is `{id, kind, writable, ...}` where `kind` is one of:

| kind | extra fields | value to send |
|---|---|---|
| `toggle` | | `true` / `false` |
| `range` | `min`, `max`, `step`, `unit` | number |
| `select` | `options` | one of `options` |
| `trigger` | | none |
| `cover` | `options` (`open`, `close`, `stop`) | one of `options` |
| `readout` | `unit` | read only |
| `stream` | `url` | read only |

Errors are `{"ok": false, "error": {"code", "message"}}` with codes `unauthorized`,
`not_found`, `unsupported`, `invalid_value`, `read_only`, `home_assistant_error`.

A client only knows the kinds above, so a new device needs no client change. Supporting a new
Home Assistant domain means adding one entry to `domainCatalog` in `mapping.go`.

## Notes

- Areas and hidden entities come from Home Assistant's template API, which only admin users
  may call. For a non-admin user the list still works but has no areas.
- State is polled by clients; there is no push channel yet.
