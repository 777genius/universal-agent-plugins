"""Contract checks for bounded native Kimi 2.5.2 MCP management evidence."""
import unittest
from unittest.mock import patch
from pathlib import Path

import kimi_252_native_web_mcp_probe as native


class KimiNativeWebProbeTests(unittest.TestCase):
    def test_rejects_wrong_project_attribution(self):
        path = Path("/private/project/.kimi-code/mcp.json")
        valid = {"name": native.SERVER_NAME, "source": "global",
                 "origin": str(path), "mutable": False}
        native.ensure_project_server({native.SERVER_NAME: valid}, path)
        for changed in ({"origin": "/foreign/mcp.json"}, {"mutable": True}, {"source": "plugin"}):
            broken = {**valid, **changed}
            with self.assertRaises(ValueError):
                native.ensure_project_server({native.SERVER_NAME: broken}, path)
        with self.assertRaises(ValueError):
            native.ensure_project_server({}, path)

    @patch.object(native, "response")
    def test_list_shape_and_token_required(self, api):
        api.return_value = [{"name": native.SERVER_NAME, "source": "global"}]
        rows = native.list_servers("http://127.0.0.1:12345", "secret", Path("/a"))
        self.assertIn(native.SERVER_NAME, rows)
        api.return_value = {"name": native.SERVER_NAME}
        with self.assertRaises(ValueError):
            native.list_servers("http://127.0.0.1:12345", "secret", Path("/a"))

    def test_never_queries_a_non_loopback_address(self):
        with self.assertRaises(ValueError):
            native.response("/api/v1/meta", "http://127.0.0.2:1", "abc")
        with self.assertRaises(ValueError):
            native.response("/api/v1/meta", "https://127.0.0.1:1", "abc")


if __name__ == "__main__":
    unittest.main()
