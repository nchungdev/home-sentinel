import json
import os
import pathlib
import tempfile
import unittest
from unittest.mock import patch, MagicMock

import providers


class ProvidersTest(unittest.TestCase):
    def test_provider_registry_loading(self):
        conf = {
            "default_provider": "gemini",
            "providers": {
                "gemini": {
                    "type": "gemini_api",
                    "api_key": "fake_key",
                    "model": "gemini-1.5-flash"
                },
                "agy": {
                    "type": "cli",
                    "command": "agy",
                    "flags": ["-p"]
                },
                "local": {
                    "type": "openai_compatible",
                    "url": "http://127.0.0.1:11434/v1/chat/completions"
                }
            },
            "fallback_order": ["gemini", "agy", "local"]
        }
        with tempfile.NamedTemporaryFile("w+", suffix=".json", delete=False) as f:
            json.dump(conf, f)
            conf_path = f.name

        try:
            reg = providers.ProviderRegistry(conf_path)
            self.assertEqual("gemini", reg.default_name)
            self.assertIn("gemini", reg.providers)
            self.assertIn("agy", reg.providers)
            self.assertIn("local", reg.providers)
            self.assertEqual("gemini", reg.choose_provider("chào bạn").name)
            self.assertEqual("agy", reg.choose_provider("dùng agy kiểm tra hệ thống").name)
            self.assertEqual("local", reg.choose_provider("xài local thôi").name)
        finally:
            os.remove(conf_path)

    def test_provider_quota_fallback(self):
        reg = providers.ProviderRegistry()
        p1 = MagicMock()
        p1.name = "primary"
        p1.is_available.return_value = True
        p1.stream.side_effect = RuntimeError("QUOTA_ERROR: 429 ResourceExhausted")

        p2 = MagicMock()
        p2.name = "fallback"
        p2.is_available.return_value = True
        p2.stream.side_effect = lambda p, s, sp, t, send: send("Kết quả từ fallback")

        reg.providers = {"primary": p1, "fallback": p2}
        reg.default_name = "primary"
        reg.fallback_order = ["primary", "fallback"]

        pieces = []
        chosen = reg.route_and_stream("test", "", "", 10, pieces.append)
        self.assertEqual("fallback", chosen)
        self.assertEqual("Kết quả từ fallback", "".join(pieces))
        p1.mark_quota_exhausted.assert_called()


if __name__ == "__main__":
    unittest.main()
