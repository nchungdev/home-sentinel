# Agent server for the AI Agent card

The launcher's **AI Agent** card listens on the TV and sends what it heard to `agent_server.py`. Run that on any
machine the TV can reach: a home server, a NAS, a mini PC, a VPS. All it needs is Python 3 and a CLI that is
logged in (`claude` by default). The server runs the CLI for each request and streams the answer back; a follow-up in
the same popup continues the conversation. Nothing ties it to one machine: the TV only needs the right address.

## Set up on the server

As the user the CLI runs under:

```bash
mkdir -p ~/tv-agent ~/.config/systemd/user
cp agent_server.py ~/tv-agent/
cp tv-agent.service ~/.config/systemd/user/
umask 077
printf 'AGENT_TOKEN=%s\nAGENT_PORT=8788\nAGENT_CMD=%s\nAGENT_WORKDIR=%s\n' \
  "$(openssl rand -hex 24)" "$(command -v claude)" "$HOME" > ~/.config/tv-agent.env
systemctl --user daemon-reload && systemctl --user enable --now tv-agent
```

Without systemd, run `AGENT_TOKEN=... python3 agent_server.py` (it reads the same variables).
Set `AGENT_ALLOW` in `~/.config/tv-agent.env` to the TV's and your phone's IPs to refuse every other client.
`AGENT_WORKDIR` is the directory the CLI works in: point it at a folder you are happy for the agent to touch.
`AGENT_CMD` can be another CLI as long as it takes `-p` and the stream-json flags used in `run_cli`.

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
