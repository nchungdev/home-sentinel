# Home Sentinel server setup

The launcher's **AI Agent** card listens on the TV and sends what it heard to the Sentinel Go backend (`bin/sentinel_server`).
Run that on any machine the TV can reach: a home server, a NAS, a mini PC, a VPS.

## Set up on the server

Run these commands from the repository root as the service user:

```bash
mkdir -p ~/.config/systemd/user
cp deploy/systemd/tv-agent.service ~/.config/systemd/user/
umask 077
printf 'AGENT_TOKEN=%s\nAGENT_PORT=8788\n' \
  "$(openssl rand -hex 24)" > ~/.config/tv-agent.env
systemctl --user daemon-reload && systemctl --user enable --now tv-agent
```

Without systemd, run `AGENT_TOKEN=... ./bin/sentinel_server` (it reads the same variables).
Set `AGENT_ALLOW` in `~/.config/tv-agent.env` to the TV's and your phone's IPs to refuse every other client.
`AGENT_WORKDIR` is the directory the CLI works in: point it at a folder you are happy for the agent to touch.
`AGENT_CMD` can be another CLI as long as it takes `-p` and the stream-json flags used in `run_cli`.

## YouTube video search

The media search uses the public YouTube Data API v3. Create an API key in Google Cloud,
enable YouTube Data API v3, and restrict the key to that API. Keep the key on the server;
do not put it in the web client or commit it.

For the systemd service, add this line to `~/.config/tv-agent.env`:

```text
YOUTUBE_API_KEY=your-key
```

Keep the file private and restart the service:

```bash
chmod 600 ~/.config/tv-agent.env
systemctl --user restart tv-agent
```

For a directly launched development server, set `YOUTUBE_API_KEY` in its process
environment before starting it. The source reports as unconfigured until the server restarts
with the key. This enables public video search; it does not use or personalize by a Google
account.

See the [YouTube Data API search documentation](https://developers.google.com/youtube/v3/docs/search/list).

## Point the TV at it

Put the server's address in `apps/launcher/app/src/main/res/values/config.xml` (`agent_url`) and its host in
`res/xml/network_security.xml` (cleartext HTTP is allowed only to that host). Build and install the launcher, then give
the app the token and the microphone:

```bash
TOKEN=$(ssh USER@SERVER 'grep ^AGENT_TOKEN= ~/.config/tv-agent.env | cut -d= -f2')
echo "$TOKEN" > /tmp/agent_token
adb -s TV_IP:5555 shell mkdir -p /sdcard/Android/data/com.gathin.tvhome/files
adb -s TV_IP:5555 push /tmp/agent_token /sdcard/Android/data/com.gathin.tvhome/files/agent_token
adb -s TV_IP:5555 shell pm grant com.gathin.tvhome android.permission.RECORD_AUDIO
rm /tmp/agent_token
```

To move to another machine, deploy the same files there, change `agent_url` and the host in
`network_security.xml`, rebuild, and push the new token.

## API

`POST /ask` with `Authorization: Bearer <token>` and `{"text": "...", "session": "id"}`; the answer streams back
as chunked `text/plain`. `GET /health` answers `ok` (401 for a wrong token). `GET /` is a phone page: the QR on the
overlay opens `http://SERVER:8788/#<token>`, and the token in the fragment is sent only as the header.

## Security

- The token is the only credential. Keep the env file at mode 600 and the port on your LAN, not on the internet.
- `claude -p` does not write files or run commands without permission. To let it, add `--allowedTools ...` in
  `run_cli`, and keep `AGENT_WORKDIR` narrow.
- The token lives in the app's external files dir on the TV, not in the APK.

## Rich answers and provider failures

The TV renders GFM Markdown and sanitized static HTML in a local WebView. Tables, HTTPS
images, links and static `card`, `grid`, `badge` layouts are supported. Source cards show
link labels and domains; Open Graph metadata fetching and arbitrary scripts/widgets are
not supported. Parser and sanitizer are bundled, so formatting does not require a CDN.
Only HTTP(S) links open externally; no JavaScript-to-native bridge is exposed.

The default style allows useful Markdown. `AGENT_STYLE` can override it. The phone page
still presents the stream as plain text. TTS strips markup and link targets separately.

Quota/credit failures return an ordinary Vietnamese message without calling another AI.
Nonzero exits, result errors, partial-stream failures and timeouts also produce a service
message. Internal stderr is inspected only for classification and is not returned to clients.
These messages report unavailability, rather than answering the original question.

Run provider failure tests with `python3 -B -m unittest discover -s services/agent` from the
repository root. See `docs/nas-ai-capacity.md` for measured NAS resources and model estimates.

TV renderer QA uses the release instrumentation APK (`assembleReleaseAndroidTest`) and
`adb shell am instrument -w com.gathin.tvhome.test/com.gathin.tvhome.AgentRenderInstrumentation`.
It feeds sample content into the real TV WebView, checks the DOM/image decode, and saves
screenshots in the app's external files directory. It does not call an AI provider.

## Watch history

`POST /v2/watch/events` takes what the TV's apps are playing (the launcher reads their media
sessions) and keeps the newest event per app and title in `sentinel_watch_history.json`.
`GET /v2/watch/continue` answers the launcher's "continue watching" list: unfinished entries from
the last 14 days plus Plex On Deck, read from `PLEX_URL` (default `http://127.0.0.1:32400`) with
`PLEX_TOKEN` or the token in Plex Media Server's `Preferences.xml`. The contract is section 9 of
`tv-agent-launcher/docs/modules/agent-protocol.md`.
