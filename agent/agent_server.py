#!/usr/bin/env python3
import hmac
import json
import os
import re
import socket
import subprocess
import sys
import threading
import tempfile
import urllib.request
from collections import OrderedDict
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
try:
    from arbitration import AudioArbitrator
except ImportError:
    from services.agent.arbitration import AudioArbitrator

TOKEN = os.environ.get("AGENT_TOKEN", "")
PORT = int(os.environ.get("AGENT_PORT", "8788"))
WORKDIR = os.environ.get("AGENT_WORKDIR", os.path.expanduser("~"))
COMMAND = os.environ.get("AGENT_CMD", "claude")
DISPLAY_STYLE = (
    "Bạn là trợ lý thông minh trên TV. "
    "Trả lời bằng tiếng Việt, súc tích, định dạng Markdown dễ đọc trên TV. "
    "Khi người dùng yêu cầu 'tìm' phim, nhạc, video hoặc nội dung, "
    "không được từ chối. Hãy chủ động gợi ý (suggest) 2-4 nội dung hay nhất, "
    "kèm tóm tắt 1 câu ngắn, năm phát hành hoặc gợi ý nơi xem (Plex/YouTube). "
    "Dùng danh sách bullet hoặc bảng để nội dung trực quan."
)
STYLE_PROMPT = os.environ.get("AGENT_STYLE", DISPLAY_STYLE)
ALLOWED_CLIENTS = {
    address.strip() for address in os.environ.get("AGENT_ALLOW", "").split(",") if address.strip()
}
TIMEOUT_SECONDS = 300
MAX_BODY_BYTES = 16 * 1024
MAX_SESSIONS = 50
MAX_CONCURRENT_REQUESTS = 2
PLAIN_TEXT = "text/plain; charset=utf-8"
HTML = "text/html; charset=utf-8"
CLI_FLAGS = ["-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages"]
DISCOVERY_PORT = int(os.environ.get("AGENT_DISCOVERY_PORT", "8789"))
DISCOVERY_PROBE = b"TV_AGENT_DISCOVER"
NOTIFY_PORT = int(os.environ.get("AGENT_NOTIFY_PORT", "8790"))
JSON_TYPE = "application/json; charset=utf-8"
LOCAL_LLM_URL = os.environ.get(
    "AGENT_LOCAL_LLM_URL", "http://127.0.0.1:9931/v1/chat/completions"
)
MAX_LOAD_1M = float(os.environ.get("AGENT_MAX_LOAD", "4.0"))
MIN_MEM_AVAILABLE_MB = float(os.environ.get("AGENT_MIN_MEM_MB", "2048.0"))

REQUEST_SLOTS = threading.BoundedSemaphore(MAX_CONCURRENT_REQUESTS)
CLI_SESSIONS = OrderedDict()
CLI_SESSIONS_LOCK = threading.Lock()
ARBITRATOR = AudioArbitrator()

PHONE_PAGE = """<!doctype html><meta charset=utf-8>
<meta name=viewport content='width=device-width,initial-scale=1'><title>AI Agent</title>
<style>
body{font:18px sans-serif;margin:16px;background:#101418;color:#e6e6e6}
textarea,button{font:inherit;width:100%;box-sizing:border-box;margin:6px 0;padding:10px;
border-radius:10px;border:1px solid #555;background:#1b2128;color:#fff}
button{background:#6750a4;border:0}
pre{white-space:pre-wrap}
</style>
<h3>AI Agent</h3>
<textarea id=question rows=4 placeholder='Gõ hoặc bấm mic trên bàn phím'></textarea>
<button id=send>Gửi</button>
<pre id=output></pre>
<script>
const token = location.hash.slice(1);
const session = Math.random().toString(36).slice(2);
send.onclick = async () => {
  const text = question.value.trim();
  if (!text) return;
  output.textContent = '…';
  send.disabled = true;
  try {
    const response = await fetch('/ask', {
      method: 'POST',
      headers: {'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'},
      body: JSON.stringify({text, session}),
    });
    if (!response.ok) {
      output.textContent = await response.text();
      return;
    }
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    output.textContent = '';
    for (;;) {
      const {done, value} = await reader.read();
      if (done) break;
      output.textContent += decoder.decode(value, {stream: true});
    }
    question.value = '';
  } catch (error) {
    output.textContent = String(error);
  } finally {
    send.disabled = false;
  }
};
</script>"""


def text_delta(event):
    if event.get("type") != "stream_event":
        return None

    inner = event.get("event") or {}
    if inner.get("type") != "content_block_delta":
        return None

    delta = inner.get("delta") or {}
    return delta.get("text") if delta.get("type") == "text_delta" else None


def remember_session(session, cli_session):
    with CLI_SESSIONS_LOCK:
        CLI_SESSIONS[session] = cli_session
        CLI_SESSIONS.move_to_end(session)
        while len(CLI_SESSIONS) > MAX_SESSIONS:
            CLI_SESSIONS.popitem(last=False)


def resume_arguments(session):
    with CLI_SESSIONS_LOCK:
        cli_session = CLI_SESSIONS.get(session) if session else None
    return ["--resume", cli_session] if cli_session else []


def style_arguments():
    return ["--append-system-prompt", STYLE_PROMPT] if STYLE_PROMPT else []


def start_cli(session, errors):
    return subprocess.Popen(
        [COMMAND] + CLI_FLAGS + style_arguments() + resume_arguments(session),
        cwd=WORKDIR,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=errors,
        text=True,
        encoding="utf-8",
    )


QUOTA_MESSAGE = (
    "Dịch vụ AI hiện đã hết hạn mức hoặc số dư. "
    "Máy chủ vẫn hoạt động, nhưng chưa thể tạo câu trả lời mới. "
    "Bạn hãy thử lại sau khi hạn mức được khôi phục."
)
FAILURE_MESSAGE = "AI tạm thời không phản hồi. Bạn hãy thử lại sau."
TIMEOUT_MESSAGE = (
    "AI trả lời quá lâu nên yêu cầu đã dừng. "
    "Bạn hãy thử câu hỏi ngắn hơn."
)
OVERLOAD_MESSAGE = (
    "Máy chủ hiện đang bận xử lý tác vụ media (Plex/Jellyfin) hoặc quá tải. "
    "Yêu cầu AI đã tạm hoãn để đảm bảo hệ thống. "
    "Bạn hãy thử lại sau ít phút."
)
QUOTA_MARKERS = (
    "rate_limit", "rate limit", "usage limit", "hit your limit", "credit balance",
    "insufficient_quota", "insufficient credits", "quota", "out of extra usage",
    "usage_limit", "too many requests", "hit your limit", "hit the limit",
    "you have hit", "you've hit",
)


def failure_message(detail, timed_out=False):
    if timed_out:
        return TIMEOUT_MESSAGE
    if any(marker in detail.lower() for marker in QUOTA_MARKERS):
        return QUOTA_MESSAGE
    return FAILURE_MESSAGE


def assistant_text(event):
    message = event.get("message") or {}
    return "".join(
        block.get("text", "") for block in message.get("content", [])
        if block.get("type") == "text"
    )


def system_overloaded():
    try:
        load1, _, _ = os.getloadavg()
        if load1 > MAX_LOAD_1M:
            return True
    except OSError:
        pass

    try:
        with open("/proc/meminfo", "r") as info:
            for line in info:
                if line.startswith("MemAvailable:"):
                    avail_mb = int(line.split()[1]) / 1024.0
                    if avail_mb < MIN_MEM_AVAILABLE_MB:
                        return True
                    break
    except OSError:
        pass

    return False


def stream_local_llm(text, send_piece):
    messages = []
    if STYLE_PROMPT:
        messages.append({"role": "system", "content": STYLE_PROMPT})
    messages.append({"role": "user", "content": text})

    payload = json.dumps({
        "messages": messages,
        "stream": True,
        "temperature": 0.3,
        "max_tokens": 1024,
    }).encode("utf-8")

    request = urllib.request.Request(
        LOCAL_LLM_URL,
        data=payload,
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=TIMEOUT_SECONDS) as response:
        for raw_line in response:
            line = raw_line.decode("utf-8").strip()
            if not line.startswith("data: ") or line == "data: [DONE]":
                continue
            chunk = json.loads(line[6:])
            choices = chunk.get("choices") or []
            if not choices:
                continue
            delta = choices[0].get("delta") or {}
            content = delta.get("content")
            if content:
                send_piece(content)


def normalize_voice_input(text):
    result = text
    actions = r"(mở|vào|bật|tìm|xem|app|ứng dụng)"
    plex_mistakes = r"(black|flash|flex|phách|bách)"
    patterns = [
        (rf"(?i)\b{actions}\s+{plex_mistakes}\b", r"\1 Plex"),
        (rf"(?i)^\s*{plex_mistakes}(\s+(server|app))?\s*$", "Plex"),
        (r"(?i)\b(dê\s*ly\s*fin|jelly\s*fin|gelllyfin)\b", "Jellyfin"),
        (r"(?i)\b(nét\s*phờ\s*lích|net\s*flick)\b", "Netflix"),
        (r"(?i)\b(du\s*túp|du\s*tuýp)\b", "YouTube"),
    ]
    for pattern, replacement in patterns:
        result = re.sub(pattern, replacement, result)
    return result


TV_SCRIPT = os.path.expanduser(os.environ.get("AGENT_TV_SCRIPT", "~/bin/tv"))


try:
    import media_bridge
except ImportError:
    media_bridge = None

try:
    import providers
    CONFIG_PATH = os.environ.get(
        "AGENT_CONFIG",
        os.path.join(os.path.dirname(__file__), "agent_config.json")
    )
    REGISTRY = providers.ProviderRegistry(CONFIG_PATH)
except ImportError:
    providers = None
    REGISTRY = None


def try_tv_command(text):
    if not os.path.isfile(TV_SCRIPT) or not os.access(TV_SCRIPT, os.X_OK):
        return None

    cleaned = text.strip().lower()
    watch_yt = re.search(
        r"^(xem|mở)\s+(video\s+|clip\s+)?(.*?)"
        r"(\s+trên\s+youtube|\s+ở\s+youtube)?$",
        cleaned,
    )
    if watch_yt:
        target = watch_yt.group(3).strip()
        has_yt = bool(watch_yt.group(4)) or "youtube" in cleaned
        if target and (has_yt or "video" in cleaned or "clip" in cleaned):
            try:
                subprocess.run(
                    [TV_SCRIPT, "search_youtube", target],
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                    timeout=5,
                    check=False,
                )
                return f"Đang mở '{target}' trên YouTube..."
            except Exception:
                return "Không thể mở YouTube lúc này."

    open_prefix = r"((mở|bật|vào)\s+)?"
    mapping = [
        (rf"^{open_prefix}plex$", ["open", "plex"], "Đang mở Plex trên TV..."),
        (rf"^{open_prefix}youtube$", ["open", "youtube"], "Đang mở YouTube trên TV..."),
        (rf"^{open_prefix}netflix$", ["open", "netflix"], "Đang mở Netflix trên TV..."),
        (r"^(về\s+)?(màn\s+hình\s+chính|home)$", ["open", "home"], "Đã về Home."),
        (r"^(tăng\s+âm( lượng)?)$", ["key", "volume_up"], "Đã tăng âm lượng."),
        (r"^(giảm\s+âm( lượng)?)$", ["key", "volume_down"], "Đã giảm âm lượng."),
        (r"^(tắt\s+tiếng|mute)$", ["key", "mute"], "Đã tắt/bật tiếng."),
        (r"^(tạm\s+dừng|tiếp\s+tục|pause|play)$", ["key", "play_pause"], "Đã gửi."),
    ]
    for pattern, args, reply in mapping:
        if re.search(pattern, cleaned):
            try:
                subprocess.run(
                    [TV_SCRIPT] + args,
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                    timeout=5,
                    check=False,
                )
                return reply
            except Exception:
                return "Không thể gửi lệnh tới TV lúc này."
    return None


def try_smarthome_command(text):
    cleaned = text.strip().lower()
    keywords = [
        "đèn", "quạt", "máy lạnh", "điều hòa", "rèm",
        "đi ngủ", "xem phim", "tiếp khách",
        "tắt hết", "bật hết", "trạng thái nhà", "ngôi nhà"
    ]
    if not any(k in cleaned for k in keywords):
        return None

    hub_url = os.environ.get("AGENT_HUB_URL", "http://127.0.0.1:8888")
    target = hub_url.rstrip("/") + "/api/hub/agent/chat"
    req_body = json.dumps({"text": text}).encode("utf-8")
    req = urllib.request.Request(
        target,
        data=req_body,
        headers={"Content-Type": "application/json"}
    )
    try:
        with urllib.request.urlopen(req, timeout=3.0) as resp:
            if resp.status == 200:
                data = json.loads(resp.read().decode("utf-8"))
                if data.get("ok") and "reply" in data:
                    reply_obj = data["reply"]
                    content = reply_obj.get("content", "")
                    if content and not content.startswith("Tôi đã ghi nhận:"):
                        return content
    except Exception:
        pass
    return None


def run_cli(text, session, send_piece, user_pref=None):
    text = normalize_voice_input(text)
    handled_tv = try_tv_command(text)
    if handled_tv:
        send_piece(handled_tv)
        return

    handled_smarthome = try_smarthome_command(text)
    if handled_smarthome:
        send_piece(handled_smarthome)
        return

    if media_bridge:
        download_match = re.search(
            r"^(tải|download|down)\s+(phim\s+)?(.*?)$", text.strip(), re.I
        )
        if download_match:
            movie_name = download_match.group(3).strip()
            if movie_name:
                send_piece(media_bridge.add_to_radarr(movie_name))
                return

        search_match = re.search(
            r"^(tìm\s+kiếm\s+phim|tìm\s+phim|phim\s+gợi\s+ý|tìm\s+kiếm)\s+(.*?)$",
            text.strip(),
            re.I,
        )
        if search_match:
            query = search_match.group(2).strip()
            cards = media_bridge.render_media_results(query)
            if cards:
                send_piece(cards)
                return

    if system_overloaded():
        send_piece(OVERLOAD_MESSAGE)
        return

    if REGISTRY and os.path.isfile(CONFIG_PATH):
        try:
            REGISTRY.route_and_stream(
                text, session, STYLE_PROMPT, TIMEOUT_SECONDS, send_piece, user_pref
            )
            return
        except Exception as err:
            err_msg = str(err)
            if providers and providers.is_quota_error(err_msg):
                send_piece(QUOTA_MESSAGE)
            elif "timeout" in err_msg.lower():
                send_piece(TIMEOUT_MESSAGE)
            else:
                send_piece(FAILURE_MESSAGE)
            return

    if COMMAND == "local":
        try:
            stream_local_llm(text, send_piece)
        except Exception:
            send_piece(FAILURE_MESSAGE)
        return

    with tempfile.TemporaryFile(mode="w+", encoding="utf-8") as errors:
        process = start_cli(session, errors)
        expired = threading.Event()

        def expire():
            expired.set()
            process.kill()

        watchdog = threading.Timer(TIMEOUT_SECONDS, expire)
        watchdog.start()
        streamed = False
        final_answer = ""
        failure = ""
        try:
            process.stdin.write(text)
            process.stdin.close()
            for raw_line in process.stdout:
                try:
                    event = json.loads(raw_line)
                except ValueError:
                    continue

                piece = text_delta(event)
                if piece:
                    streamed = True
                    send_piece(piece)
                if event.get("session_id") and session and not event.get("is_error"):
                    remember_session(session, event["session_id"])
                if event.get("type") == "assistant":
                    final_answer = assistant_text(event) or final_answer
                    if event.get("error"):
                        failure = str(event.get("error")) + final_answer
                if event.get("type") == "result":
                    final_answer = event.get("result") or final_answer
                    if event.get("is_error"):
                        failure += json.dumps(event.get("errors", [])) + final_answer
            process.wait()
        finally:
            watchdog.cancel()
            if process.poll() is None:
                process.kill()
            process.wait()
            process.stdout.close()

        errors.seek(0)
        diagnostic = errors.read(16384)
        if failure or process.returncode != 0 or expired.is_set():
            notice = failure_message(failure + diagnostic, expired.is_set())
            send_piece(("\n\n" if streamed else "") + notice)
        elif not streamed:
            send_piece(final_answer or FAILURE_MESSAGE)


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, format, *arguments):
        sys.stderr.write("%s %s\n" % (self.client_address[0], format % arguments))

    def client_allowed(self):
        return not ALLOWED_CLIENTS or self.client_address[0] in ALLOWED_CLIENTS

    def token_valid(self):
        sent = self.headers.get("Authorization", "")
        return sent.startswith("Bearer ") and hmac.compare_digest(
            sent[len("Bearer "):].encode(), TOKEN.encode()
        )

    def reply(self, code, content_type, body):
        data = body.encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if not self.client_allowed():
            return self.reply(403, PLAIN_TEXT, "forbidden")

        if self.path == "/health":
            if self.headers.get("Authorization") and not self.token_valid():
                return self.reply(401, PLAIN_TEXT, "unauthorised")
            return self.reply(200, PLAIN_TEXT, "ok")

        if self.path.split("?")[0] == "/":
            return self.reply(200, HTML, PHONE_PAGE)

        self.reply(404, PLAIN_TEXT, "not found")

    def read_request(self):
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            length = -1
        if length <= 0 or length > MAX_BODY_BYTES:
            self.reply(413, PLAIN_TEXT, "bad length")
            return None

        try:
            body = json.loads(self.rfile.read(length).decode("utf-8"))
        except (ValueError, UnicodeDecodeError):
            self.reply(400, PLAIN_TEXT, "bad json")
            return None

        text = str(body.get("text", "")).strip()
        if not text:
            self.reply(400, PLAIN_TEXT, "empty")
            return None
        pref = str(body.get("provider", "")).strip() or None
        return text, str(body.get("session", ""))[:64], pref

    def read_notification(self):
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            length = -1
        if length <= 0 or length > MAX_BODY_BYTES:
            self.reply(413, PLAIN_TEXT, "bad length")
            return None

        try:
            body = json.loads(self.rfile.read(length).decode("utf-8"))
        except (ValueError, UnicodeDecodeError):
            self.reply(400, PLAIN_TEXT, "bad json")
            return None

        if not isinstance(body, dict):
            self.reply(400, PLAIN_TEXT, "bad json object")
            return None

        title = str(body.get("title", "")).strip()
        message = str(body.get("message", "")).strip()
        if not title and not message:
            self.reply(400, PLAIN_TEXT, "empty notification")
            return None

        return {
            "type": str(body.get("type", "glance")),
            "title": title,
            "message": message,
            "tts": bool(body.get("tts", False)),
            "timeout_seconds": max(1, int(body.get("timeout_seconds", 15))),
        }

    def start_stream(self):
        self.send_response(200)
        self.send_header("Content-Type", PLAIN_TEXT)
        self.send_header("Transfer-Encoding", "chunked")
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Accel-Buffering", "no")
        self.end_headers()

    def send_chunk(self, piece):
        data = piece.encode("utf-8")
        self.wfile.write(b"%x\r\n" % len(data) + data + b"\r\n")
        self.wfile.flush()

    def stream_answer(self, text, session, user_pref=None):
        self.start_stream()
        try:
            run_cli(text, session, self.send_chunk, user_pref)
        except (BrokenPipeError, ConnectionResetError):
            return
        except Exception:
            try:
                self.send_chunk("\n" + FAILURE_MESSAGE)
            except OSError:
                return
        self.wfile.write(b"0\r\n\r\n")
        self.wfile.flush()

    def handle_notify(self):
        if not self.token_valid():
            return self.reply(401, PLAIN_TEXT, "unauthorised")
        notification = self.read_notification()
        if notification is None:
            return
        broadcast_notification(notification)
        self.reply(200, JSON_TYPE, json.dumps({"status": "ok"}))

    def handle_ask(self):
        if not self.token_valid():
            return self.reply(401, PLAIN_TEXT, "unauthorised")
        request = self.read_request()
        if request is None:
            return
        if not REQUEST_SLOTS.acquire(blocking=False):
            return self.reply(429, PLAIN_TEXT, "busy, try again")
        try:
            self.stream_answer(*request)
        finally:
            REQUEST_SLOTS.release()

    def read_json_payload(self):
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            length = -1
        if length <= 0 or length > MAX_BODY_BYTES:
            self.reply(413, PLAIN_TEXT, "bad length")
            return None
        try:
            return json.loads(self.rfile.read(length).decode("utf-8"))
        except (ValueError, UnicodeDecodeError):
            self.reply(400, PLAIN_TEXT, "bad json")
            return None

    def handle_config_camera(self):
        body = self.read_json_payload()
        if body is None:
            return
        host = str(body.get("host", "")).strip()
        port = int(body.get("port", 554))
        stream_url = str(body.get("stream_url", "")).strip()
        if not host:
            response = {"status": "error", "message": "IP camera không được để trống"}
            return self.reply(400, JSON_TYPE, json.dumps(response))
        success, message = verify_camera_connection(host, port)
        if not success:
            response = {"status": "error", "message": message}
            return self.reply(400, JSON_TYPE, json.dumps(response))
        save_camera_configuration(stream_url)
        response = {"status": "ok", "message": "Xác thực và lưu camera thành công"}
        self.reply(200, JSON_TYPE, json.dumps(response))

    def handle_config_smarthome(self):
        body = self.read_json_payload()
        if body is None:
            return
        server_url = str(body.get("server_url", "")).strip()
        token = str(body.get("token", "")).strip()
        auto_wake = bool(body.get("auto_wake", True))
        auto_sleep = bool(body.get("auto_sleep", True))
        greeting = bool(body.get("greeting", True))
        if not server_url:
            msg = "URL máy chủ không được để trống"
            return self.reply(400, JSON_TYPE, json.dumps({"status": "error", "message": msg}))
        success, message = verify_smarthome_connection(server_url, token)
        if not success:
            response = {"status": "error", "message": message}
            return self.reply(400, JSON_TYPE, json.dumps(response))
        save_smarthome_configuration(server_url, token, auto_wake, auto_sleep, greeting)
        response = {"status": "ok", "message": "Xác thực Home Assistant thành công"}
        self.reply(200, JSON_TYPE, json.dumps(response))

    def handle_arbitration(self):
        body = self.read_json_payload()
        if body is None:
            return
        decision = ARBITRATOR.arbitrate(body)
        self.reply(200, JSON_TYPE, json.dumps(decision))

    def do_POST(self):
        if not self.client_allowed():
            return self.reply(403, PLAIN_TEXT, "forbidden")

        if self.path == "/notify":
            return self.handle_notify()

        if self.path == "/ask":
            return self.handle_ask()

        if self.path == "/api/config/camera":
            return self.handle_config_camera()

        if self.path == "/api/config/smarthome":
            return self.handle_config_smarthome()

        if self.path == "/api/arbitration/decide":
            return self.handle_arbitration()

        self.reply(404, PLAIN_TEXT, "not found")


def broadcast_notification(notification):
    data = json.dumps(notification).encode("utf-8")
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)
    target = os.environ.get("AGENT_BROADCAST_IP", "255.255.255.255")
    try:
        sock.sendto(data, (target, NOTIFY_PORT))
    except Exception as error:
        sys.stderr.write("failed to broadcast notification: %s\n" % error)
    finally:
        sock.close()


def verify_camera_connection(host, port):
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(3.0)
    try:
        sock.connect((host, port))
        if port == 554:
            probe = f"OPTIONS rtsp://{host}:{port}/ RTSP/1.0\r\nCSeq: 1\r\n\r\n"
            sock.sendall(probe.encode("utf-8"))
            resp = sock.recv(256).decode("utf-8", errors="ignore")
            if resp and "RTSP/1.0" in resp:
                return True, "Kết nối cổng RTSP thành công"
        return True, f"Kết nối cổng {port} thành công"
    except socket.timeout:
        return False, f"Hết thời gian kết nối tới {host}:{port}"
    except Exception as error:
        return False, f"Không thể kết nối tới {host}:{port}: {error}"
    finally:
        sock.close()


def verify_smarthome_connection(server_url, token):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    candidates = ["/api/", "/health", "/api/devices", "/api/hub/overview"]
    for path in candidates:
        target = server_url.rstrip("/") + path
        request = urllib.request.Request(target, headers=headers)
        try:
            with urllib.request.urlopen(request, timeout=3.0) as response:
                if response.status == 200:
                    return True, "Kết nối Smarthome / Home Assistant thành công"
        except urllib.error.HTTPError as error:
            if error.code == 401:
                return False, "Token không chính xác (401)"
        except Exception:
            pass
    return False, f"Không thể kết nối tới {server_url}"


def save_camera_configuration(stream_url):
    base_dir = os.path.dirname(os.path.abspath(__file__))
    config_path = os.path.join(base_dir, "..", "camera", "camera_config.json")
    config = {}
    if os.path.exists(config_path):
        try:
            with open(config_path, "r", encoding="utf-8") as f:
                config = json.load(f)
        except Exception:
            config = {}
    config["stream_url"] = stream_url
    try:
        with open(config_path, "w", encoding="utf-8") as f:
            json.dump(config, f, indent=2)
    except Exception as error:
        sys.stderr.write("failed to save camera config: %s\n" % error)


def save_smarthome_configuration(server_url, token, auto_wake, auto_sleep, greeting):
    base_dir = os.path.dirname(os.path.abspath(__file__))
    config_path = os.path.join(base_dir, "..", "smarthome", "smarthome_config.json")
    config = {}
    if os.path.exists(config_path):
        try:
            with open(config_path, "r", encoding="utf-8") as f:
                config = json.load(f)
        except Exception:
            config = {}
    config["home_assistant"] = {"url": server_url, "token": token}
    rules = config.get("rules", {})
    rules["on_sofa_enter"] = {
        "tv_action": "wake" if auto_wake else "none",
        "greeting": "Xin chào! Chúc bạn xem phim vui vẻ." if greeting else ""
    }
    rules["on_sofa_leave"] = {
        "tv_action": "sleep" if auto_sleep else "none"
    }
    config["rules"] = rules
    try:
        with open(config_path, "w", encoding="utf-8") as f:
            json.dump(config, f, indent=2)
    except Exception as error:
        sys.stderr.write("failed to save smarthome config: %s\n" % error)


def start_discovery_responder():
    def loop():
        try:
            listener = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
            listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            listener.bind(("0.0.0.0", DISCOVERY_PORT))
            while True:
                data, address = listener.recvfrom(128)
                if data.strip() == DISCOVERY_PROBE:
                    payload = json.dumps({"port": PORT}).encode("utf-8")
                    listener.sendto(payload, address)
        except Exception as error:
            sys.stderr.write("discovery responder stopped: %s\n" % error)

    thread = threading.Thread(target=loop, daemon=True)
    thread.start()


def main():
    if not TOKEN:
        sys.exit("AGENT_TOKEN is not set")

    start_discovery_responder()
    server = ThreadingHTTPServer(("0.0.0.0", PORT), Handler)
    sys.stderr.write("tv-agent listening on :%d, cwd %s, cmd %s\n" % (PORT, WORKDIR, COMMAND))
    server.serve_forever()


if __name__ == "__main__":
    main()
