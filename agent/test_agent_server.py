import importlib.util
import json
import os
import pathlib
import socket
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location(
    "agent_server", pathlib.Path(__file__).with_name("agent_server.py")
)
server = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(server)


class AgentFailureTest(unittest.TestCase):
    def run_script(self, body, timeout=5):
        with tempfile.TemporaryDirectory() as folder:
            script = pathlib.Path(folder) / "fake-cli"
            script.write_text("#!/usr/bin/env python3\n" + body)
            script.chmod(0o700)
            pieces = []
            with patch.object(server, "COMMAND", str(script)), \
                    patch.object(server, "TIMEOUT_SECONDS", timeout), \
                    patch.object(server, "system_overloaded", return_value=False):
                server.run_cli("hello", "", pieces.append)
            return "".join(pieces)

    def test_quota_without_model_response(self):
        answer = self.run_script(
            'import sys\nsys.stderr.write("Credit balance is too low")\nsys.exit(1)\n'
        )
        self.assertEqual(server.QUOTA_MESSAGE, answer)

    def test_result_quota(self):
        answer = self.run_script(
            'print(\'{"type":"result","is_error":true,'
            '"errors":["rate_limit_error"]}\')\n'
        )
        self.assertEqual(server.QUOTA_MESSAGE, answer)

    def test_assistant_quota_survives_final_result(self):
        answer = self.run_script(
            'print(\'{"type":"assistant","error":"rate_limit","message":{"content":[]}}\')\n'
            'print(\'{"type":"result","is_error":true,"result":"Unavailable"}\')\n'
        )
        self.assertEqual(server.QUOTA_MESSAGE, answer)

    def test_successful_result(self):
        answer = self.run_script('print(\'{"type":"result","result":"Xin chào"}\')\n')
        self.assertEqual("Xin chào", answer)

    def test_partial_then_failure(self):
        answer = self.run_script(
            'import json,sys\nprint(json.dumps({"type":"stream_event","event":{'
            '"type":"content_block_delta","delta":{"type":"text_delta","text":"Một phần"}'
            '}}))\nsys.exit(1)\n'
        )
        self.assertEqual("Một phần\n\n" + server.FAILURE_MESSAGE, answer)

    def test_timeout(self):
        answer = self.run_script('import time\ntime.sleep(2)\n', timeout=0.05)
        self.assertEqual(server.TIMEOUT_MESSAGE, answer)

    def test_internal_errors_are_not_exposed(self):
        answer = self.run_script('import sys\nsys.stderr.write("private detail")\nsys.exit(1)')
        self.assertEqual(server.FAILURE_MESSAGE, answer)

    def test_overload_blocks_execution(self):
        pieces = []
        with patch.object(server, "system_overloaded", return_value=True):
            server.run_cli("xin chào", "", pieces.append)
        self.assertEqual(server.OVERLOAD_MESSAGE, "".join(pieces))

    def test_normalize_voice_input(self):
        self.assertEqual("mở Plex", server.normalize_voice_input("mở black"))
        self.assertEqual("xem Plex", server.normalize_voice_input("xem flash"))
        self.assertEqual("Plex", server.normalize_voice_input("flash"))
        self.assertEqual("Plex", server.normalize_voice_input("black server"))
        self.assertEqual("mở Jellyfin", server.normalize_voice_input("mở dê ly fin"))

    def test_try_tv_command(self):
        with tempfile.NamedTemporaryFile(mode="w", delete=False) as f:
            f.write("#!/bin/sh\nexit 0\n")
            path = f.name
        os.chmod(path, 0o755)
        try:
            with patch.object(server, "TV_SCRIPT", path):
                self.assertEqual(
                    "Đang mở Plex trên TV...", server.try_tv_command("mở plex")
                )
                self.assertIsNone(server.try_tv_command("hôm nay ăn gì"))
        finally:
            os.remove(path)

    def test_local_llm_mode(self):
        fake_sse = (
            'data: {"choices":[{"delta":{"content":"Chào"}}]}\n\n'
            'data: {"choices":[{"delta":{"content":" bạn"}}]}\n\n'
            'data: [DONE]\n\n'
        ).encode("utf-8")

        class FakeResponse:
            def __enter__(self):
                return self
            def __exit__(self, *args):
                pass
            def __iter__(self):
                return iter(fake_sse.splitlines(keepends=True))

        pieces = []
        with patch.object(server, "COMMAND", "local"), \
                patch.object(server, "system_overloaded", return_value=False), \
                patch("urllib.request.urlopen", return_value=FakeResponse()):
            server.run_cli("test", "", pieces.append)
        self.assertEqual("Chào bạn", "".join(pieces))

    def test_broadcast_notification(self):
        receiver = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        receiver.bind(("127.0.0.1", 0))
        port = receiver.getsockname()[1]
        with patch.object(server, "NOTIFY_PORT", port), \
                patch.dict("os.environ", {"AGENT_BROADCAST_IP": "127.0.0.1"}):
            payload = {"type": "greeting", "title": "Xin chào", "message": "Chào bạn"}
            server.broadcast_notification(payload)
            data, _ = receiver.recvfrom(1024)
            received = json.loads(data.decode("utf-8"))
            self.assertEqual("greeting", received.get("type"))
            self.assertEqual("Xin chào", received.get("title"))
        receiver.close()

    def test_verify_camera_failure(self):
        success, message = server.verify_camera_connection("127.0.0.1", 59999)
        self.assertFalse(success)

    def test_arbitration_remote_mic_override(self):
        result = server.ARBITRATOR.arbitrate({"remote_mic_active": True, "tv_screen_on": False})
        self.assertEqual("tv", result["listening_device"])

    def test_arbitration_tv_on(self):
        result = server.ARBITRATOR.arbitrate({"tv_screen_on": True})
        self.assertEqual("tv", result["listening_device"])
        self.assertEqual("tv", result["speaking_device"])

    def test_arbitration_tv_off_user_on_sofa(self):
        result = server.ARBITRATOR.arbitrate({"tv_screen_on": False, "user_on_sofa": True})
        self.assertEqual("camera", result["listening_device"])
        self.assertEqual("camera", result["speaking_device"])


if __name__ == "__main__":
    unittest.main()
