import json
import os
import re
import subprocess
import tempfile
import threading
import urllib.parse
import urllib.request

QUOTA_MARKERS = (
    "rate_limit", "rate limit", "usage limit", "hit your limit", "credit balance",
    "insufficient_quota", "insufficient credits", "quota", "out of extra usage",
    "usage_limit", "too many requests", "hit the limit", "you have hit",
    "you've hit", "resource_exhausted", "resourceexhausted", "429"
)

def is_quota_error(text):
    if not text:
        return False
    lowered = text.lower()
    return any(marker in lowered for marker in QUOTA_MARKERS)


class BaseProvider:
    def __init__(self, name, config):
        self.name = name
        self.config = config
        self.disabled_until = 0

    def is_available(self):
        import time
        return time.time() >= self.disabled_until

    def mark_quota_exhausted(self, cooldown_seconds=300):
        import time
        self.disabled_until = time.time() + cooldown_seconds

    def stream(self, prompt, session, style_prompt, timeout_seconds, send_piece):
        raise NotImplementedError


class CliProvider(BaseProvider):
    def __init__(self, name, config):
        super().__init__(name, config)
        self.command = config.get("command", name)
        self.flags = config.get("flags", ["-p", "--output-format", "stream-json"])
        self.workdir = config.get("workdir", os.path.expanduser("~"))

    def stream(self, prompt, session, style_prompt, timeout_seconds, send_piece):
        if os.path.basename(self.command) != "claude":
            raise RuntimeError("CLI provider chưa hỗ trợ giới hạn công cụ an toàn")
        cmd = [self.command] + list(self.flags) + [
            "--tools", "", "--strict-mcp-config", "--mcp-config", '{"mcpServers":{}}',
            "--setting-sources", "", "--settings", '{"disableAllHooks":true}',
        ]
        if self.command in ("agy", "antigravity"):
            if style_prompt:
                cmd.extend(["--append-system-prompt", style_prompt])
            if session:
                cmd.extend(["--conversation", session])
        elif self.command == "claude":
            if style_prompt:
                cmd.extend(["--append-system-prompt", style_prompt])
            if session:
                cmd.extend(["--resume", session])

        with tempfile.TemporaryFile(mode="w+", encoding="utf-8") as errors:
            process = subprocess.Popen(
                cmd,
                cwd=self.workdir,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=errors,
                text=True,
                encoding="utf-8",
            )
            expired = threading.Event()

            def expire():
                expired.set()
                process.kill()

            watchdog = threading.Timer(timeout_seconds, expire)
            watchdog.start()
            streamed = False
            final_answer = ""
            try:
                process.stdin.write(prompt)
                process.stdin.close()
                for raw_line in process.stdout:
                    try:
                        event = json.loads(raw_line)
                    except ValueError:
                        continue

                    piece = None
                    if event.get("type") == "stream_event":
                        inner = event.get("event") or {}
                        delta = inner.get("delta") or {}
                        if delta.get("type") == "text_delta":
                            piece = delta.get("text")
                    elif event.get("type") == "assistant":
                        msg = event.get("message") or {}
                        content = msg.get("content") or []
                        piece = "".join(
                            b.get("text", "") for b in content if b.get("type") == "text"
                        )

                    if piece:
                        if is_quota_error(piece):
                            self.mark_quota_exhausted()
                            raise RuntimeError(f"QUOTA_ERROR: {piece}")
                        streamed = True
                        send_piece(piece)

                    if event.get("result"):
                        final_answer = event["result"]

                process.wait()
                if expired.is_set():
                    raise TimeoutError("CLI timeout")

                if process.returncode != 0:
                    errors.seek(0)
                    err_msg = errors.read()
                    if is_quota_error(err_msg):
                        self.mark_quota_exhausted()
                        raise RuntimeError(f"QUOTA_ERROR: {err_msg}")
                    raise RuntimeError(f"CLI exit {process.returncode}: {err_msg}")

                if not streamed and final_answer:
                    send_piece(final_answer)
            finally:
                watchdog.cancel()
                if process.poll() is None:
                    process.kill()


class GeminiApiProvider(BaseProvider):
    def __init__(self, name, config):
        super().__init__(name, config)
        self.api_key = os.path.expandvars(config.get("api_key", ""))
        self.model = config.get("model", "gemini-1.5-flash")

    def stream(self, prompt, session, style_prompt, timeout_seconds, send_piece):
        key = self.api_key or os.environ.get("GEMINI_API_KEY", "")
        if not key:
            raise RuntimeError("GEMINI_API_KEY is not configured")

        url = (
            f"https://generativelanguage.googleapis.com/v1beta/models/"
            f"{self.model}:streamGenerateContent?key={key}&alt=sse"
        )
        contents = []
        if style_prompt:
            contents.append({
                "role": "user",
                "parts": [{"text": f"System Instruction: {style_prompt}"}]
            })
            contents.append({
                "role": "model",
                "parts": [{"text": "Đã hiểu chỉ dẫn hệ thống."}]
            })
        contents.append({
            "role": "user",
            "parts": [{"text": prompt}]
        })

        payload = json.dumps({"contents": contents}).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=payload,
            headers={"Content-Type": "application/json"},
        )
        try:
            with urllib.request.urlopen(req, timeout=timeout_seconds) as resp:
                for raw_line in resp:
                    line = raw_line.decode("utf-8").strip()
                    if not line.startswith("data: "):
                        continue
                    try:
                        data = json.loads(line[6:])
                        for cand in data.get("candidates", []):
                            content = cand.get("content", {})
                            for part in content.get("parts", []):
                                text = part.get("text")
                                if text:
                                    send_piece(text)
                    except ValueError:
                        continue
        except urllib.error.HTTPError as err:
            err_body = err.read().decode("utf-8", errors="replace")
            if err.code == 429 or is_quota_error(err_body):
                self.mark_quota_exhausted()
                raise RuntimeError(f"QUOTA_ERROR: {err_body}")
            raise RuntimeError(f"HTTP {err.code}: {err_body}")


class OpenAiCompatibleProvider(BaseProvider):
    def __init__(self, name, config):
        super().__init__(name, config)
        self.url = os.path.expandvars(config.get("url", "http://127.0.0.1:11434/v1/chat/completions"))
        self.model = config.get("model", "default")
        self.api_key = os.path.expandvars(config.get("api_key", ""))

    def stream(self, prompt, session, style_prompt, timeout_seconds, send_piece):
        messages = []
        if style_prompt:
            messages.append({"role": "system", "content": style_prompt})
        messages.append({"role": "user", "content": prompt})

        payload = json.dumps({
            "model": self.model,
            "messages": messages,
            "stream": True,
            "temperature": 0.4,
            "max_tokens": 1024,
        }).encode("utf-8")

        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        req = urllib.request.Request(self.url, data=payload, headers=headers)
        try:
            with urllib.request.urlopen(req, timeout=timeout_seconds) as resp:
                for raw_line in resp:
                    line = raw_line.decode("utf-8").strip()
                    if not line.startswith("data: ") or line == "data: [DONE]":
                        continue
                    try:
                        chunk = json.loads(line[6:])
                        choices = chunk.get("choices") or []
                        if not choices:
                            continue
                        delta = choices[0].get("delta") or {}
                        content = delta.get("content")
                        if content:
                            send_piece(content)
                    except ValueError:
                        continue
        except urllib.error.HTTPError as err:
            err_body = err.read().decode("utf-8", errors="replace")
            if err.code == 429 or is_quota_error(err_body):
                self.mark_quota_exhausted()
                raise RuntimeError(f"QUOTA_ERROR: {err_body}")
            raise RuntimeError(f"HTTP {err.code}: {err_body}")


class ProviderRegistry:
    def __init__(self, config_file=None):
        self.providers = {}
        self.default_name = "auto"
        self.fallback_order = []
        if config_file and os.path.isfile(config_file):
            self.load_config(config_file)
        else:
            self.setup_defaults_from_env()

    def setup_defaults_from_env(self):
        cmd = os.environ.get("AGENT_CMD", "claude")
        if os.environ.get("GEMINI_API_KEY"):
            self.providers["gemini"] = GeminiApiProvider("gemini", {
                "api_key": os.environ.get("GEMINI_API_KEY"),
                "model": os.environ.get("GEMINI_MODEL", "gemini-1.5-flash")
            })
        if cmd == "local":
            self.providers["local"] = OpenAiCompatibleProvider("local", {
                "url": os.environ.get("AGENT_LOCAL_LLM_URL", "http://127.0.0.1:9931/v1/chat/completions")
            })
        else:
            self.providers[cmd] = CliProvider(cmd, {
                "command": cmd,
                "workdir": os.environ.get("AGENT_WORKDIR", os.path.expanduser("~"))
            })
        self.fallback_order = list(self.providers.keys())

    def load_config(self, path):
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
        self.default_name = data.get("default_provider", "auto")
        self.fallback_order = data.get("fallback_order", [])

        for name, conf in data.get("providers", {}).items():
            ptype = conf.get("type", "cli")
            if ptype == "cli":
                self.providers[name] = CliProvider(name, conf)
            elif ptype == "gemini_api":
                self.providers[name] = GeminiApiProvider(name, conf)
            elif ptype in ("openai_compatible", "local_llm"):
                self.providers[name] = OpenAiCompatibleProvider(name, conf)

        if not self.fallback_order:
            self.fallback_order = list(self.providers.keys())

    def choose_provider(self, text, user_pref=None):
        cleaned = text.lower()
        if user_pref and user_pref in self.providers and self.providers[user_pref].is_available():
            return self.providers[user_pref]

        intent_map = {
            "gemini": ["gemini", "google"],
            "agy": ["agy", "antigravity"],
            "claude": ["claude", "sonnet"],
            "local": ["local", "nội bộ", "ollama"]
        }
        for name, keywords in intent_map.items():
            if name in self.providers and self.providers[name].is_available():
                for kw in keywords:
                    if re.search(rf"\b(dùng|xài|bằng|chuyển qua|hỏi)\s+{kw}\b", cleaned):
                        return self.providers[name]

        if self.default_name != "auto" and self.default_name in self.providers:
            prov = self.providers[self.default_name]
            if prov.is_available():
                return prov

        for name in self.fallback_order:
            if name in self.providers and self.providers[name].is_available():
                return self.providers[name]

        for prov in self.providers.values():
            if prov.is_available():
                return prov

        return next(iter(self.providers.values()), None)

    def route_and_stream(self, prompt, session, style_prompt, timeout_seconds, send_piece, user_pref=None):
        preferred = self.choose_provider(prompt, user_pref)
        if not preferred:
            raise RuntimeError("No AI provider available")

        candidates = [preferred] + [
            self.providers[name] for name in self.fallback_order
            if name in self.providers and self.providers[name] != preferred and self.providers[name].is_available()
        ]

        last_error = None
        for prov in candidates:
            try:
                prov.stream(prompt, session, style_prompt, timeout_seconds, send_piece)
                return prov.name
            except Exception as err:
                last_error = err
                err_str = str(err)
                if "QUOTA_ERROR" in err_str or is_quota_error(err_str):
                    prov.mark_quota_exhausted()
                    continue
                raise err
        if last_error:
            raise last_error
